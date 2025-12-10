package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"

	"github.com/shehryarbajwa/browserbase-mini/internal/browser"
	contextmgr "github.com/shehryarbajwa/browserbase-mini/internal/context"
	"github.com/shehryarbajwa/browserbase-mini/internal/region"
	"github.com/shehryarbajwa/browserbase-mini/pkg/models"
)

// PuppeteerConnection represents a persistent Puppeteer connection
type PuppeteerConnection struct {
	SessionID string
	Process   *exec.Cmd
	Stdin     io.WriteCloser
	stdout    io.ReadCloser
	responses chan map[string]interface{}
	mu        sync.Mutex
}

// Manager handles all session operations
type Manager struct {
	sessions       sync.Map
	concurrency    map[string]*semaphore.Weighted
	puppeteerConns sync.Map // map[sessionID]*PuppeteerConnection
	mu             sync.RWMutex
	regionMgr      *region.Manager
	contextMgr     *contextmgr.Manager
}

// NewManager creates a new session manager
func NewManager(regionMgr *region.Manager, ctxMgr *contextmgr.Manager) *Manager {
	return &Manager{
		concurrency: make(map[string]*semaphore.Weighted),
		regionMgr:   regionMgr,
		contextMgr:  ctxMgr,
	}
}

// CreateSession creates a new browser session with optional context
func (m *Manager) CreateSession(ctx context.Context, req models.CreateSessionRequest) (*models.Session, error) {
	// Validate request
	if req.ProjectID == "" {
		return nil, fmt.Errorf("projectId is required")
	}

	// Apply defaults
	if req.Timeout == 0 {
		req.Timeout = 3600
	}
	if req.Timeout < 60 || req.Timeout > 21600 {
		return nil, fmt.Errorf("timeout must be between 60 and 21600 seconds")
	}
	if req.Region == "" {
		req.Region = "us-west-2"
	}

	// Check concurrency limit
	if err := m.acquireSlot(req.ProjectID); err != nil {
		return nil, err
	}

	sessionID := uuid.New().String()
	now := time.Now()

	// Route to best region
	targetRegion := m.regionMgr.RouteSession(req.Region)

	// Prepare browser options
	browserOpts := browser.LaunchBrowserOptions{
		SessionID: sessionID,
	}

	// If contextID provided, verify it exists and try to load data
	if req.ContextID != "" {
		// Check if context exists
		_, err := m.contextMgr.GetContext(req.ContextID)
		if err != nil {
			m.releaseSlot(req.ProjectID)
			return nil, fmt.Errorf("context not found: %w", err)
		}

		// Try to load context data (might be empty if first use)
		userDataDir, err := m.contextMgr.LoadContextData(req.ContextID)
		if err != nil {
			// Context exists but has no data yet - create fresh directory
			userDataDir = fmt.Sprintf("/tmp/browser-context-%s", req.ContextID)
			if err := os.MkdirAll(userDataDir, 0755); err != nil {
				m.releaseSlot(req.ProjectID)
				return nil, fmt.Errorf("failed to create context directory: %w", err)
			}
		}
		browserOpts.UserDataDir = userDataDir
	}

	// Launch browser with or without context
	var browserInstance *browser.BrowserInstance
	var err error

	if browserOpts.UserDataDir != "" {
		// Launch with context
		browserInstance, err = m.regionMgr.LaunchBrowserWithOptions(ctx, targetRegion, browserOpts)
	} else {
		// Launch without context (simple)
		browserInstance, err = m.regionMgr.LaunchBrowser(ctx, targetRegion, sessionID)
	}

	if err != nil {
		m.releaseSlot(req.ProjectID)
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	// Create session
	session := &models.Session{
		ID:          sessionID,
		ProjectID:   req.ProjectID,
		Region:      string(targetRegion),
		Status:      models.StatusRunning,
		StartedAt:   now,
		ExpiresAt:   now.Add(time.Duration(req.Timeout) * time.Second),
		Timeout:     req.Timeout,
		ConnectURL:  browserInstance.ConnectURL,
		ContainerID: browserInstance.ContainerID,
		ContextID:   req.ContextID,
		UserDataDir: browserInstance.UserDataDir,
	}

	// Store session
	m.sessions.Store(session.ID, session)

	// Start persistent Puppeteer connection
	if err := m.startPuppeteerConnection(session); err != nil {
		log.Printf("⚠️ Failed to start Puppeteer connection: %v", err)
		// Don't fail session creation, just log it
	}

	// Start timeout handler
	go m.handleTimeout(session)

	return session, nil
}

// startPuppeteerConnection creates a persistent Node.js process for this session
func (m *Manager) startPuppeteerConnection(session *models.Session) error {
	// Path to puppeteer script
	scriptPath := "./internal/session/puppeteer.js"

	// Check if file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("puppeteer script not found at %s", scriptPath)
	}

	// START NODE PROCESS with script file
	cmd := exec.Command("node", scriptPath, session.ConnectURL)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe failed: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe failed: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start puppeteer: %w", err)
	}

	conn := &PuppeteerConnection{
		SessionID: session.ID,
		Process:   cmd,
		Stdin:     stdin,
		stdout:    stdout,
		responses: make(chan map[string]interface{}, 10),
	}

	// READ STDOUT → JSON messages
	go func() {
		scanner := bufio.NewScanner(stdout)

		// ✅ INCREASE BUFFER SIZE for large screenshots
		buf := make([]byte, 0, 64*1024)   // Start with 64KB
		scanner.Buffer(buf, 10*1024*1024) // Max 10MB

		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("PUPPETEER[%s] OUT: %s", session.ID[:8], line)

			var result map[string]interface{}
			if json.Unmarshal([]byte(line), &result) == nil {
				select {
				case conn.responses <- result:
				default:
					log.Printf("⚠️ Response buffer full for %s", session.ID)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Scanner error for %s: %v", session.ID[:8], err)
		}
	}()

	// STDERR logs
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("PUPPETEER[%s] ERR: %s", session.ID[:8], scanner.Text())
		}
	}()

	// WAIT FOR READY
	select {
	case msg := <-conn.responses:
		if msg["status"] != "ready" {
			return fmt.Errorf("puppeteer failed to initialize: %v", msg)
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		return fmt.Errorf("puppeteer startup timeout")
	}

	m.puppeteerConns.Store(session.ID, conn)
	log.Printf("✅ Puppeteer connected for session %s", session.ID[:8])
	return nil
}

// GetPuppeteerConnection retrieves the persistent Puppeteer connection for a session
func (m *Manager) GetPuppeteerConnection(sessionID string) *PuppeteerConnection {
	value, ok := m.puppeteerConns.Load(sessionID)
	if !ok {
		return nil
	}
	return value.(*PuppeteerConnection)
}

// SendCommand sends a command to the Puppeteer process and waits for response
func (conn *PuppeteerConnection) SendCommand(cmd map[string]string, timeout time.Duration) (map[string]interface{}, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	// Send command
	if _, err := conn.Stdin.Write(append(cmdJSON, '\n')); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Wait for response
	select {
	case response := <-conn.responses:
		if status, ok := response["status"].(string); ok && status == "error" {
			return nil, fmt.Errorf("puppeteer error: %v", response["message"])
		}
		return response, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("command timeout")
	}
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(id string) (*models.Session, error) {
	value, ok := m.sessions.Load(id)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return value.(*models.Session), nil
}

// ListSessions returns all sessions for a project, optionally filtered by status
func (m *Manager) ListSessions(projectID string, status models.SessionStatus) []*models.Session {
	var sessions []*models.Session

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*models.Session)

		if projectID != "" && session.ProjectID != projectID {
			return true
		}

		if status != "" && session.Status != status {
			return true
		}

		sessions = append(sessions, session)
		return true
	})

	return sessions
}

// DeleteSession marks a session as completed and optionally saves context
func (m *Manager) DeleteSession(id string) error {
	session, err := m.GetSession(id)
	if err != nil {
		return err
	}

	if session.Status != models.StatusRunning {
		return fmt.Errorf("session is not running")
	}

	// Close Puppeteer connection first
	if conn := m.GetPuppeteerConnection(id); conn != nil {
		log.Printf("🔌 Closing Puppeteer connection for session %s", id[:8])
		conn.SendCommand(map[string]string{"action": "close"}, 5*time.Second)
		conn.Process.Wait()
		m.puppeteerConns.Delete(id)
	}

	// Save context if this session was using one
	if session.ContextID != "" && session.UserDataDir != "" {
		if err := m.saveSessionContext(session); err != nil {
			fmt.Printf("Warning: failed to save context %s: %v\n", session.ContextID, err)
		}
	}

	// Stop the Docker container
	if session.ContainerID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := m.regionMgr.StopBrowser(ctx, session.ContainerID); err != nil {
			fmt.Printf("Warning: failed to stop container %s: %v\n", session.ContainerID, err)
		}
	}

	// Update status
	session.Status = models.StatusCompleted
	m.sessions.Store(id, session)

	// Release concurrency slot
	m.releaseSlot(session.ProjectID)

	return nil
}

// saveSessionContext saves the browser's user data directory to the context
func (m *Manager) saveSessionContext(session *models.Session) error {
	// Save the data
	if err := m.contextMgr.SaveContextData(session.ContextID, session.UserDataDir); err != nil {
		return err
	}

	// Update context timestamp
	return m.contextMgr.UpdateContext(session.ContextID)
}

// acquireSlot tries to acquire a concurrency slot for the project
func (m *Manager) acquireSlot(projectID string) error {
	m.mu.Lock()
	sem, exists := m.concurrency[projectID]
	if !exists {
		sem = semaphore.NewWeighted(10)
		m.concurrency[projectID] = sem
	}
	m.mu.Unlock()

	if !sem.TryAcquire(1) {
		return fmt.Errorf("concurrency limit reached for project %s", projectID)
	}

	return nil
}

// releaseSlot releases a concurrency slot for the project
func (m *Manager) releaseSlot(projectID string) {
	m.mu.RLock()
	sem := m.concurrency[projectID]
	m.mu.RUnlock()

	if sem != nil {
		sem.Release(1)
	}
}

// handleTimeout automatically terminates a session after its timeout
func (m *Manager) handleTimeout(session *models.Session) {
	timer := time.NewTimer(time.Duration(session.Timeout) * time.Second)
	defer timer.Stop()

	<-timer.C

	current, err := m.GetSession(session.ID)
	if err != nil {
		return
	}

	if current.Status != models.StatusRunning {
		return
	}

	// Close Puppeteer connection
	if conn := m.GetPuppeteerConnection(current.ID); conn != nil {
		log.Printf("🔌 Closing Puppeteer connection for timed out session %s", current.ID[:8])
		conn.SendCommand(map[string]string{"action": "close"}, 5*time.Second)
		conn.Process.Wait()
		m.puppeteerConns.Delete(current.ID)
	}

	// Save context if this session was using one
	if current.ContextID != "" && current.UserDataDir != "" {
		if err := m.saveSessionContext(current); err != nil {
			fmt.Printf("Warning: failed to save context %s on timeout: %v\n", current.ContextID, err)
		}
	}

	// Stop the browser container
	if current.ContainerID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := m.regionMgr.StopBrowser(ctx, current.ContainerID); err != nil {
			fmt.Printf("Warning: failed to stop container %s on timeout: %v\n", current.ContainerID, err)
		}
	}

	// Update status
	current.Status = models.StatusTimedOut
	m.sessions.Store(current.ID, current)
	m.releaseSlot(current.ProjectID)
}
