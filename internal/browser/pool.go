package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type BrowserInstance struct {
	ContainerID string
	SessionID   string
	ConnectURL  string
	Region      string
	Port        string
	UserDataDir string
}

type Pool struct {
	client   *client.Client
	region   string
	basePort int
}

func NewPool(region string, basePort int) (*Pool, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Pool{
		client:   cli,
		region:   region,
		basePort: basePort,
	}, nil
}

type LaunchBrowserOptions struct {
	SessionID   string
	UserDataDir string
}

func (p *Pool) LaunchBrowser(ctx context.Context, sessionID string) (*BrowserInstance, error) {
	return p.LaunchBrowserWithOptions(ctx, LaunchBrowserOptions{
		SessionID: sessionID,
	})
}

func (p *Pool) LaunchBrowserWithOptions(ctx context.Context, opts LaunchBrowserOptions) (*BrowserInstance, error) {
	userDataDir := opts.UserDataDir
	if userDataDir == "" {
		userDataDir = filepath.Join(os.TempDir(), "browser-data", opts.SessionID)
		if err := os.MkdirAll(userDataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create user data directory: %w", err)
		}
	}

	// Use browserless/chrome - it just works!
	containerConfig := &container.Config{
		Image: "browserless/chrome:latest",
		Labels: map[string]string{
			"session-id": opts.SessionID,
			"region":     p.region,
			"managed-by": "browserbase-mini",
		},
		Env: []string{
			"CONNECTION_TIMEOUT=-1",        // Disable connection timeout
			"MAX_CONCURRENT_SESSIONS=1",    // Only allow 1 session per container
			"PREBOOT_CHROME=true",          // Pre-boot Chrome for faster startup
			"KEEP_ALIVE=true",              // Keep connections alive
			"EXIT_ON_HEALTH_FAILURE=false", // Don't exit on health check failures
		},
		ExposedPorts: nat.PortSet{
			"3000/tcp": struct{}{},
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"3000/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0",
				},
			},
		},
		AutoRemove: false,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: userDataDir,
				Target: "/data",
			},
		},
	}

	resp, err := p.client.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		fmt.Sprintf("session-%s", opts.SessionID[:8]),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := p.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for container to be ready
	inspect, err := p.client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	port := inspect.NetworkSettings.Ports["3000/tcp"][0].HostPort

	// Wait for the browser to be ready by checking the /json/version endpoint
	if err := p.waitForBrowserReady(port); err != nil {
		return nil, fmt.Errorf("browser failed to become ready: %w", err)
	}

	instance := &BrowserInstance{
		ContainerID: resp.ID,
		SessionID:   opts.SessionID,
		ConnectURL:  fmt.Sprintf("ws://localhost:%s", port),
		Region:      p.region,
		Port:        port,
		UserDataDir: userDataDir,
	}

	return instance, nil
}

func (p *Pool) StopBrowser(ctx context.Context, containerID string) error {
	timeout := 10
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}

	if err := p.client.ContainerStop(ctx, containerID, stopOptions); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	if err := p.client.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

func (p *Pool) IsHealthy(ctx context.Context, containerID string) bool {
	inspect, err := p.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return false
	}
	return inspect.State.Running
}

func (p *Pool) EnsureImage(ctx context.Context) error {
	images, err := p.client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return err
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == "browserless/chrome:latest" {
				return nil
			}
		}
	}

	reader, err := p.client.ImagePull(ctx, "browserless/chrome:latest", image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	_, err = io.Copy(io.Discard, reader)
	return err
}

func (p *Pool) Close() error {
	return p.client.Close()
}

// waitForBrowserReady waits for the browser to be ready by checking the /json/version endpoint
func (p *Pool) waitForBrowserReady(port string) error {
	url := fmt.Sprintf("http://localhost:%s/json/version", port)
	maxRetries := 20 // 10 seconds total (20 * 500ms)

	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				// Give it a bit more time for WebSocket to be fully ready
				time.Sleep(500 * time.Millisecond)
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("browser did not become ready after %d retries", maxRetries)
}
