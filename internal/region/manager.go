package region

import (
	"context"
	"fmt"
	"sync"

	"github.com/shehryarbajwa/browserbase-mini/internal/browser"
)

// Region represents a geographical region
type Region string

const (
	RegionUSWest2    Region = "us-west-2"
	RegionUSEast1    Region = "us-east-1"
	RegionEUCentral1 Region = "eu-central-1"
)

// RegionalPool wraps a browser pool with region metadata
type RegionalPool struct {
	Region Region
	Pool   *browser.Pool
	Port   int
}

// Manager manages browser pools across multiple regions
type Manager struct {
	pools map[Region]*RegionalPool
	mu    sync.RWMutex
}

// NewManager creates a new multi-region manager
func NewManager() (*Manager, error) {
	manager := &Manager{
		pools: make(map[Region]*RegionalPool),
	}

	// Initialize pools for each region
	regions := []struct {
		region Region
		port   int
	}{
		{RegionUSWest2, 9222},
		{RegionUSEast1, 9322},
		{RegionEUCentral1, 9422},
	}

	for _, r := range regions {
		pool, err := browser.NewPool(string(r.region), r.port)
		if err != nil {
			return nil, fmt.Errorf("failed to create pool for %s: %w", r.region, err)
		}

		manager.pools[r.region] = &RegionalPool{
			Region: r.region,
			Pool:   pool,
			Port:   r.port,
		}
	}

	return manager, nil
}

// GetPool returns the browser pool for a specific region
func (m *Manager) GetPool(region Region) (*browser.Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	regionalPool, exists := m.pools[region]
	if !exists {
		return nil, fmt.Errorf("unsupported region: %s", region)
	}

	return regionalPool.Pool, nil
}

// RouteSession determines the best region for a session
func (m *Manager) RouteSession(requestedRegion string) Region {
	region := Region(requestedRegion)

	m.mu.RLock()
	_, exists := m.pools[region]
	m.mu.RUnlock()

	if exists {
		return region
	}

	return RegionUSWest2
}

// LaunchBrowser launches a browser in the specified region
func (m *Manager) LaunchBrowser(ctx context.Context, region Region, sessionID string) (*browser.BrowserInstance, error) {
	pool, err := m.GetPool(region)
	if err != nil {
		return nil, err
	}

	return pool.LaunchBrowser(ctx, sessionID)
}

// LaunchBrowserWithOptions launches a browser with custom options
func (m *Manager) LaunchBrowserWithOptions(ctx context.Context, region Region, opts browser.LaunchBrowserOptions) (*browser.BrowserInstance, error) {
	pool, err := m.GetPool(region)
	if err != nil {
		return nil, err
	}

	return pool.LaunchBrowserWithOptions(ctx, opts)
}

// StopBrowser stops a browser in any region
func (m *Manager) StopBrowser(ctx context.Context, containerID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for _, regionalPool := range m.pools {
		err := regionalPool.Pool.StopBrowser(ctx, containerID)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return lastErr
}

// EnsureImages ensures Chrome image is available in all regions
func (m *Manager) EnsureImages(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for region, regionalPool := range m.pools {
		if err := regionalPool.Pool.EnsureImage(ctx); err != nil {
			return fmt.Errorf("failed to ensure image in %s: %w", region, err)
		}
	}

	return nil
}

// GetRegions returns all available regions
func (m *Manager) GetRegions() []Region {
	m.mu.RLock()
	defer m.mu.RUnlock()

	regions := make([]Region, 0, len(m.pools))
	for region := range m.pools {
		regions = append(regions, region)
	}

	return regions
}

// Close closes all browser pools
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, regionalPool := range m.pools {
		if err := regionalPool.Pool.Close(); err != nil {
			return err
		}
	}

	return nil
}
