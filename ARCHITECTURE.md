# Architecture Documentation

## Table of Contents

1. [System Overview](#system-overview)
2. [Technology Stack](#technology-stack)
3. [Component Architecture](#component-architecture)
4. [Data Flow](#data-flow)
5. [Core Components](#core-components)
6. [Storage Strategy](#storage-strategy)
7. [Concurrency & Rate Limiting](#concurrency--rate-limiting)
8. [Multi-Region Architecture](#multi-region-architecture)
9. [Session Lifecycle](#session-lifecycle)
10. [Security Considerations](#security-considerations)

## System Overview

Browserbase Mini is a microservices-style browser automation platform built around containerized Chrome instances. The system follows a layered architecture:

```
┌─────────────────────────────────────────────────┐
│           Presentation Layer                     │
│  (React Frontend + REST API)                     │
└───────────────┬─────────────────────────────────┘
                │
┌───────────────┼─────────────────────────────────┐
│           Business Logic Layer                   │
│  • Session Management                            │
│  • Context Persistence                           │
│  • Region Routing                                │
│  • Rate Limiting                                 │
└───────────────┬─────────────────────────────────┘
                │
┌───────────────┼─────────────────────────────────┐
│           Infrastructure Layer                   │
│  • Docker Container Management                   │
│  • Puppeteer Bridge (Node.js)                    │
│  • WebSocket Proxy                               │
└─────────────────────────────────────────────────┘
```

## Technology Stack

### Backend (Go)

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **HTTP Router** | Gorilla Mux | RESTful routing |
| **WebSocket** | Gorilla WebSocket | CDP proxy |
| **Container SDK** | Docker Go SDK | Docker management |
| **Concurrency** | golang.org/x/sync | Semaphores for limits |
| **Rate Limiting** | golang.org/x/time/rate | Token bucket algorithm |
| **UUID** | google/uuid | Session identifiers |
| **Environment** | godotenv | .env configuration |

### Browser Automation

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Browser** | browserless/chrome | Headless Chrome in Docker |
| **Automation** | Puppeteer Core | Chrome DevTools Protocol |
| **IPC Bridge** | Node.js subprocess | Go ↔ Puppeteer communication |

### Frontend

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Framework** | React 19.2.0 | UI components |
| **HTTP Client** | Axios 1.13.2 | API communication |
| **Build Tool** | Vite 7.2.4 | Fast development |
| **Linting** | ESLint 9.39.1 | Code quality |

## Component Architecture

### High-Level Component Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                          Client Layer                        │
│  ┌────────────────┐              ┌────────────────┐         │
│  │  React UI      │◄────HTTP────►│  HTTP Client   │         │
│  │  (Frontend)    │              │  (curl, SDK)   │         │
│  └────────────────┘              └────────────────┘         │
└────────────────────────────┬────────────────────────────────┘
                             │
                             │ HTTP/WebSocket
                             │
┌────────────────────────────┼────────────────────────────────┐
│                    API Gateway (Port 8080)                   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Gorilla Mux Router                                  │   │
│  │  • CORS Middleware                                   │   │
│  │  • Rate Limit Middleware (per projectId)            │   │
│  │  • Request Logging                                   │   │
│  └──────────────────────────────────────────────────────┘   │
└────────────────────────────┬────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Session    │    │   Context    │    │   Region     │
│   Manager    │◄──►│   Manager    │    │   Manager    │
│              │    │              │    │              │
│ • Create     │    │ • Save       │    │ • Route      │
│ • List       │    │ • Load       │    │ • Pool Mgmt  │
│ • Get        │    │ • Delete     │    │ • Failover   │
│ • Delete     │    │ • Compress   │    │              │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                    │
       │                   │                    │
       └───────────────────┼────────────────────┘
                           │
                           ▼
               ┌────────────────────────┐
               │    Browser Pool        │
               │  (Docker Container)    │
               │                        │
               │  • Launch              │
               │  • Stop                │
               │  • Health Check        │
               │  • Port Allocation     │
               └────────┬───────────────┘
                        │
                        │ Spawns
                        │
        ┌───────────────┼───────────────┐
        │               │               │
        ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│  Container   │ │  Container   │ │  Container   │
│  us-west-2   │ │  us-east-1   │ │ eu-central-1 │
│  :9222       │ │  :9322       │ │  :9422       │
└──────┬───────┘ └──────┬───────┘ └──────┬───────┘
       │                │                │
       │ CDP            │ CDP            │ CDP
       │                │                │
       └────────────────┼────────────────┘
                        │
                        ▼
            ┌────────────────────────┐
            │   Puppeteer Bridge     │
            │   (Node.js Subprocess) │
            │                        │
            │  • puppeteer.js        │
            │  • Stdin/Stdout IPC    │
            │  • JSON Commands       │
            └────────────────────────┘
```

## Data Flow

### 1. Session Creation Flow

```
┌──────────┐
│  Client  │
└────┬─────┘
     │
     │ POST /v1/sessions
     │ {projectId, region, timeout, contextId}
     │
     ▼
┌────────────────────┐
│  Rate Limiter      │ Check: 100 req/hour limit
└────┬───────────────┘
     │ ✓ Allowed
     │
     ▼
┌────────────────────┐
│  Session Handler   │ Validate request
└────┬───────────────┘
     │
     │ CreateSession()
     │
     ▼
┌────────────────────┐
│  Session Manager   │ Check: Max 10 concurrent sessions
└────┬───────────────┘
     │ ✓ Capacity available
     │
     │ RouteToRegion(region)
     │
     ▼
┌────────────────────┐
│  Region Manager    │ Select regional pool (us-west-2)
└────┬───────────────┘
     │
     │ LaunchBrowser()
     │
     ▼
┌────────────────────┐
│  Browser Pool      │ 1. Pull browserless/chrome image
│                    │ 2. Create Docker container
│                    │ 3. Mount /data volume
│                    │ 4. Expose CDP port (9222)
└────┬───────────────┘
     │ Container ID + Port
     │
     ▼
┌────────────────────┐
│  Session Manager   │ 5. Spawn Node.js process (puppeteer.js)
│                    │ 6. Connect Puppeteer to ws://localhost:9222
└────┬───────────────┘
     │
     │ (Optional) LoadContext(contextId)
     │
     ▼
┌────────────────────┐
│  Context Manager   │ 7. Extract tar.gz to /tmp/context-{id}
│                    │ 8. Configure browser to use context dir
└────┬───────────────┘
     │ Context loaded
     │
     ▼
┌────────────────────┐
│  Session Manager   │ 9. Store session metadata
│                    │ 10. Start timeout timer
└────┬───────────────┘
     │
     │ Session object
     │
     ▼
┌────────────────────┐
│  HTTP Response     │
│  {                 │
│    id,             │
│    status: RUNNING,│
│    debuggerUrl,    │
│    ...             │
│  }                 │
└────────────────────┘
```

### 2. Screenshot Capture Flow

```
Client
  │
  │ GET /v1/sessions/{id}/screenshot
  │
  ▼
Session Handler
  │
  │ GetPuppeteerConnection(sessionId)
  │
  ▼
Session Manager ──────► Puppeteer Process (stdin)
  │                     │
  │                     │ JSON: {"action": "screenshot"}
  │                     │
  │                     ▼
  │                  Execute page.screenshot({encoding: 'base64'})
  │                     │
  │                     │ Capture PNG
  │                     │
  │◄────────────────────┘
  │ Base64 PNG (stdout)
  │
  ▼
HTTP Response
  {
    "screenshot": "iVBORw0KGg..."
  }
```

### 3. Context Persistence Flow

```
Session Termination
  │
  │ DELETE /v1/sessions/{id}
  │
  ▼
Session Manager
  │
  │ 1. Send "close" command to Puppeteer
  │
  ▼
Puppeteer Process
  │
  │ 2. browser.close() - Flush data to /data
  │ 3. Exit gracefully
  │
  ▼
Session Manager
  │
  │ SaveSessionContext(sessionId)
  │
  ▼
Context Manager
  │
  │ 4. Create tar.gz from Docker volume /data
  │ 5. Save to ./storage/contexts/{contextId}.tar.gz
  │
  ▼
Browser Pool
  │
  │ 6. Stop Docker container
  │ 7. Remove container
  │ 8. Cleanup volumes
  │
  ▼
Session Manager
  │
  │ 9. Remove session from in-memory map
  │
  ▼
HTTP Response: 200 OK
```

## Core Components

### 1. Session Manager (`internal/session/manager.go`)

**Responsibilities:**
- Session lifecycle (create, read, delete)
- Puppeteer process management
- Timeout enforcement
- Connection pooling

**Key Data Structures:**
```go
type Manager struct {
    sessions    sync.Map              // sessionId -> *models.Session
    browserPool *browser.Pool
    contextMgr  *context.Manager

    // Concurrency control
    projectSemaphores sync.Map        // projectId -> *semaphore.Weighted
    maxConcurrent     int64            // 10 sessions per project
}

type Session struct {
    ID               string
    Status           string            // RUNNING, STOPPED, ERROR
    ProjectID        string
    CreatedAt        time.Time
    ExpiresAt        time.Time
    Region           string
    DebuggerURL      string
    ContainerID      string

    // Puppeteer connection
    PuppeteerCmd     *exec.Cmd
    PuppeteerStdin   io.WriteCloser
    PuppeteerStdout  io.ReadCloser
}
```

**Methods:**
- `CreateSession(req CreateSessionRequest) (*Session, error)`
- `GetSession(id string) (*Session, error)`
- `ListSessions(projectId string) ([]*Session, error)`
- `DeleteSession(id string) error`
- `GetPuppeteerConnection(id string) (*PuppeteerConnection, error)`

**Concurrency Model:**
- Uses `sync.Map` for thread-safe session storage
- Semaphore per project (10 concurrent sessions)
- Goroutine per session for timeout monitoring

### 2. Browser Pool (`internal/browser/pool.go`)

**Responsibilities:**
- Docker container lifecycle
- Image management
- Port allocation
- Health checks

**Key Operations:**
```go
type Pool struct {
    client    *docker.Client
    imageName string               // "browserless/chrome:latest"
    region    string
}

func (p *Pool) LaunchBrowser(contextPath string) (*LaunchResult, error) {
    // 1. Pull image if not exists
    // 2. Create container with:
    //    - Mounted volume: contextPath -> /data
    //    - Exposed port: 9222 -> random host port
    //    - Labels: region, projectId
    // 3. Start container
    // 4. Return container ID + allocated port
}

func (p *Pool) StopBrowser(containerID string) error {
    // 1. Stop container (graceful shutdown)
    // 2. Remove container
    // 3. Cleanup volumes
}
```

**Docker Configuration:**
```go
containerConfig := &container.Config{
    Image: "browserless/chrome:latest",
    Cmd:   []string{"--no-sandbox", "--disable-gpu"},
    ExposedPorts: nat.PortSet{
        "9222/tcp": struct{}{},
    },
}

hostConfig := &container.HostConfig{
    Binds: []string{
        fmt.Sprintf("%s:/data", contextPath),
    },
    PortBindings: nat.PortMap{
        "9222/tcp": []nat.PortBinding{{HostPort: "0"}}, // Auto-allocate
    },
}
```

### 3. Context Manager (`internal/context/manager.go`)

**Responsibilities:**
- Browser state persistence
- Tar.gz compression/decompression
- File system management
- Context lifecycle

**Storage Format:**
```
storage/
  contexts/
    ctx-abc123.tar.gz              # Compressed browser state
      ├── Default/
      │   ├── Cookies              # Session cookies
      │   ├── Local Storage/       # localStorage data
      │   ├── Session Storage/     # sessionStorage data
      │   └── Cache/               # Browser cache
      └── ...
```

**Methods:**
```go
type Manager struct {
    storagePath string              // ./storage/contexts
}

func (m *Manager) CreateContext(projectId string) (string, error)
func (m *Manager) SaveContextData(contextId, sourcePath string) error
func (m *Manager) LoadContextData(contextId string) (string, error)
func (m *Manager) DeleteContext(contextId string) error
```

**Compression:**
- Uses `archive/tar` + `compress/gzip`
- Preserves permissions and timestamps
- Typical size: 5-50MB per context

### 4. Region Manager (`internal/region/manager.go`)

**Responsibilities:**
- Multi-region pool management
- Region routing
- Failover logic
- Image pre-pulling

**Supported Regions:**
```go
var regions = map[string]int{
    "us-west-2":    9222,
    "us-east-1":    9322,
    "eu-central-1": 9422,
}
```

**Architecture:**
```
Region Manager
  ├── us-west-2 Pool
  │     ├── Browser 1 (Container port 9222)
  │     ├── Browser 2 (Container port 9222)
  │     └── Browser N (Container port 9222)
  │
  ├── us-east-1 Pool
  │     ├── Browser 1 (Container port 9322)
  │     └── ...
  │
  └── eu-central-1 Pool
        ├── Browser 1 (Container port 9422)
        └── ...
```

**Routing Logic:**
```go
func (rm *RegionManager) GetPool(region string) *browser.Pool {
    pool, exists := rm.pools[region]
    if !exists {
        return rm.pools["us-west-2"]  // Default fallback
    }
    return pool
}
```

### 5. Puppeteer Bridge (`internal/session/puppeteer.js`)

**Responsibilities:**
- Node.js ↔ Go IPC
- Puppeteer API wrapper
- Browser control

**Communication Protocol:**
```javascript
// Input (stdin - JSON lines)
{"action": "navigate", "url": "https://example.com"}
{"action": "screenshot"}
{"action": "close"}

// Output (stdout - JSON lines)
{"type": "ready", "debuggerUrl": "ws://..."}
{"type": "screenshot", "data": "base64..."}
{"type": "error", "message": "Navigation failed"}
```

**Implementation:**
```javascript
const puppeteer = require('puppeteer-core');

// Connect to existing Chrome instance
const browser = await puppeteer.connect({
    browserWSEndpoint: debuggerUrl
});

const page = await browser.newPage();

// Handle commands via stdin
process.stdin.on('data', async (data) => {
    const command = JSON.parse(data);

    switch(command.action) {
        case 'navigate':
            await page.goto(command.url);
            break;
        case 'screenshot':
            const screenshot = await page.screenshot({
                encoding: 'base64'
            });
            console.log(JSON.stringify({
                type: 'screenshot',
                data: screenshot
            }));
            break;
        case 'close':
            await browser.close();
            process.exit(0);
    }
});
```

### 6. Rate Limiter (`internal/api/middleware.go`)

**Implementation:**
```go
type RateLimiter struct {
    limiters sync.Map    // projectId -> *rate.Limiter
    rate     rate.Limit  // 100 requests per hour
    burst    int         // 10 burst capacity
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        projectId := r.URL.Query().Get("projectId")

        limiter := rl.getLimiter(projectId)

        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**Token Bucket Algorithm:**
- Tokens added at rate: 100/hour = 0.0278 tokens/second
- Bucket capacity: 10 tokens (burst)
- Each request consumes 1 token
- Refill is automatic and continuous

## Storage Strategy

### In-Memory Storage

**Sessions:**
- Stored in `sync.Map` (thread-safe)
- No persistence - lost on restart
- Automatic cleanup on timeout

**Rate Limiters:**
- Per-project `rate.Limiter` instances
- Reset on server restart

### File-Based Storage

**Contexts:**
- Location: `./storage/contexts/`
- Format: `{contextId}.tar.gz`
- Retention: Manual deletion only
- Size: 5-50MB per context

**Temporary Files:**
- Location: `/tmp/browser-context-{contextId}/`
- Lifetime: Session duration
- Cleanup: On session deletion

## Concurrency & Rate Limiting

### Concurrency Control

**Per-Project Session Limit:**
```go
// Maximum 10 concurrent sessions per project
const maxConcurrentSessions = 10

// Implemented using weighted semaphore
semaphore := semaphore.NewWeighted(maxConcurrentSessions)

// Acquire before creating session
if !semaphore.TryAcquire(1) {
    return errors.New("max concurrent sessions reached")
}

// Release on session deletion
defer semaphore.Release(1)
```

**Global Resource Management:**
- Docker container limit: System dependent
- Port allocation: Dynamic (0 = auto-assign)
- Memory: 100-200MB per container

### Rate Limiting

**Configuration:**
```go
// 100 requests per hour per project
rateLimiter := NewRateLimiter(
    rate.Limit(100.0 / 3600.0),  // Per second rate
    10,                           // Burst
)
```

**Applied To:**
- Session creation (POST /v1/sessions)
- Session listing (GET /v1/sessions)
- Context operations (POST /v1/contexts)

**Not Applied To:**
- Screenshot capture (GET /v1/sessions/{id}/screenshot)
- Navigation (POST /v1/sessions/{id}/navigate)
- Debug URL (GET /v1/sessions/{id}/debug)
- WebSocket connections (GET /v1/sessions/{id}/ws)

**Reasoning:** Active sessions already consume resources; rate limiting creation prevents abuse.

## Multi-Region Architecture

### Regional Isolation

Each region operates independently:

```
┌─────────────────────────────────────────────────┐
│              Region: us-west-2                   │
│  ┌────────────────────────────────────────┐    │
│  │  Browser Pool (Port 9222)              │    │
│  │  • Max 10 containers per project       │    │
│  │  • Shared storage/contexts/            │    │
│  │  • Independent timeout monitoring      │    │
│  └────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│              Region: us-east-1                   │
│  ┌────────────────────────────────────────┐    │
│  │  Browser Pool (Port 9322)              │    │
│  │  • Same project can have 10 more       │    │
│  │  • Shared storage/contexts/            │    │
│  │  • Independent timeout monitoring      │    │
│  └────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

**Key Points:**
- Concurrency limit is per-region, per-project
- Contexts are globally shared (same storage path)
- Different base ports prevent port conflicts
- Sessions in different regions are isolated

### Failover Strategy

Current: Manual region specification with default fallback

```go
if region == "" || !isValidRegion(region) {
    region = "us-west-2"  // Default
}
```

**Future Enhancement:**
- Health checks per region
- Automatic failover to healthy region
- Round-robin load balancing

## Session Lifecycle

### State Machine

```
         ┌─────────────┐
         │   PENDING   │ (brief, during container launch)
         └──────┬──────┘
                │
                │ Container started + Puppeteer connected
                │
                ▼
         ┌─────────────┐
    ┌───►│   RUNNING   │◄───┐
    │    └──────┬──────┘    │
    │           │            │
    │           │ Timeout or │ Resume
    │           │ DELETE     │ (future)
    │           │            │
    │           ▼            │
    │    ┌─────────────┐    │
    │    │  STOPPING   │────┘
    │    └──────┬──────┘
    │           │
    │           │ Context saved + Container removed
    │           │
    │           ▼
    │    ┌─────────────┐
    └────│   STOPPED   │
         └─────────────┘
                │
                │ Manual deletion
                │
                ▼
         ┌─────────────┐
         │   DELETED   │
         └─────────────┘
```

### Timeout Handling

```go
func (m *Manager) monitorTimeout(session *Session) {
    timer := time.NewTimer(session.Timeout)

    <-timer.C

    // Timeout expired - cleanup
    m.DeleteSession(session.ID)
}
```

**Timeout Range:**
- Minimum: 60 seconds
- Maximum: 21,600 seconds (6 hours)
- Default: 3,600 seconds (1 hour)

### Cleanup Process

1. Send "close" command to Puppeteer
2. Wait for graceful shutdown (5 second timeout)
3. Force kill Node.js process if necessary
4. Save context data (if configured)
5. Stop Docker container
6. Remove container and volumes
7. Delete session from memory
8. Release semaphore

## Security Considerations

### Current Implementation

**Threats Addressed:**
- Rate limiting prevents DoS
- Docker isolation prevents container escape
- No filesystem access outside mounted volumes

**Limitations (NOT PRODUCTION-READY):**
- No authentication/authorization
- No encryption (HTTP only)
- No network isolation between containers
- No resource quotas per project

### Production Hardening Recommendations

1. **Authentication:**
   ```go
   // Add API key middleware
   func AuthMiddleware(next http.Handler) http.Handler {
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           apiKey := r.Header.Get("X-API-Key")
           if !isValidAPIKey(apiKey) {
               http.Error(w, "Unauthorized", http.StatusUnauthorized)
               return
           }
           next.ServeHTTP(w, r)
       })
   }
   ```

2. **HTTPS/TLS:**
   - Enable TLS in Go HTTP server
   - Use Let's Encrypt for certificates
   - Force HTTPS redirects

3. **Docker Security:**
   - Use non-root user in containers
   - Enable AppArmor/SELinux profiles
   - Limit container capabilities
   - Use private networks per project

4. **Resource Limits:**
   - Memory limits per container (--memory=512m)
   - CPU limits (--cpus=1.0)
   - Disk quotas
   - Network bandwidth throttling

5. **Monitoring:**
   - Prometheus metrics export
   - Log aggregation (ELK stack)
   - Alerting on anomalies
   - Session audit logs

## Performance Characteristics

### Benchmarks (Approximate)

- **Session Creation:** 2-5 seconds (includes Docker pull on first run)
- **Screenshot Capture:** 100-500ms (depends on page complexity)
- **Context Save:** 500ms-2s (5-50MB tar.gz)
- **Session Deletion:** 500ms-1s (graceful shutdown)

### Scalability

**Current Limitations:**
- 10 concurrent sessions per project per region
- 100 requests/hour per project
- Single-node deployment (no horizontal scaling)

**Scaling Strategy:**
- Horizontal: Deploy multiple instances with load balancer
- Vertical: Increase Docker resources
- Distribute: Run regions on separate servers

### Resource Usage

**Per Session:**
- Memory: 150-300MB (Chrome + Node.js + Go)
- CPU: 10-20% (idle), 100% (active rendering)
- Disk: 20-50MB (browser cache + context)

**Server Requirements (10 sessions):**
- Memory: 4GB minimum, 8GB recommended
- CPU: 4 cores
- Disk: 10GB (includes Docker images)

## Extensibility

### Adding New Regions

1. Define new region in `internal/region/manager.go`:
   ```go
   regions := map[string]int{
       "ap-south-1": 9522,  // New region
   }
   ```

2. Initialize pool on startup:
   ```go
   rm.pools["ap-south-1"] = browser.NewPool(dockerClient, "ap-south-1")
   ```

### Adding New Puppeteer Commands

1. Update `internal/session/puppeteer.js`:
   ```javascript
   case 'pdf':
       const pdf = await page.pdf({format: 'A4'});
       console.log(JSON.stringify({
           type: 'pdf',
           data: pdf.toString('base64')
       }));
       break;
   ```

2. Add Go handler in `internal/api/handlers.go`:
   ```go
   func (s *Server) handlePDF(w http.ResponseWriter, r *http.Request) {
       // Send command to Puppeteer
       // Parse response
       // Return PDF
   }
   ```

3. Register route in `internal/api/server.go`:
   ```go
   router.HandleFunc("/v1/sessions/{id}/pdf", s.handlePDF).Methods("GET")
   ```

## Diagram: Complete Request Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Request                           │
│  curl -X POST http://localhost:8080/v1/sessions                 │
│  -d '{"projectId": "my-app", "region": "us-west-2"}'            │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (1) HTTP POST
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                      Gorilla Mux Router                          │
│  • Parse request                                                 │
│  • Route to /v1/sessions                                         │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (2) Check rate limit
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                   Rate Limit Middleware                          │
│  • Extract projectId                                             │
│  • Get/create rate.Limiter for project                           │
│  • Call limiter.Allow()                                          │
│  • If rate exceeded: return 429 Too Many Requests                │
└───────────────────────────┬─────────────────────────────────────┘
                            │ (3) ✓ Allowed
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                    CreateSession Handler                         │
│  • Parse JSON body                                               │
│  • Validate parameters (timeout, region)                         │
│  • Call sessionManager.CreateSession(req)                        │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (4) CreateSession(req)
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                       Session Manager                            │
│  Step 1: Check concurrency limit                                 │
│    • Get semaphore for projectId                                 │
│    • Try acquire (max 10)                                        │
│    • If full: return error "max sessions reached"                │
└───────────────────────────┬─────────────────────────────────────┘
                            │ (5) ✓ Capacity available
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                       Session Manager                            │
│  Step 2: Route to region                                         │
│    • Call regionManager.GetPool(req.Region)                      │
│    • Fallback to us-west-2 if invalid                            │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (6) GetPool("us-west-2")
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                        Region Manager                            │
│  • Lookup pool for us-west-2                                     │
│  • Return browser.Pool instance                                  │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (7) LaunchBrowser()
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                         Browser Pool                             │
│  Step 1: Ensure image exists                                     │
│    • docker.ImagePull("browserless/chrome:latest")               │
│  Step 2: Create context directory                                │
│    • mkdir /tmp/browser-context-{sessionId}                      │
│  Step 3: Create container                                        │
│    • Image: browserless/chrome:latest                            │
│    • Bind: /tmp/context -> /data                                 │
│    • Port: 9222 -> 0 (auto-allocate)                             │
│  Step 4: Start container                                         │
│    • docker.ContainerStart(containerID)                          │
│  Step 5: Inspect to get allocated port                           │
│    • docker.ContainerInspect(containerID)                        │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (8) Container running on port 54321
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                       Session Manager                            │
│  Step 3: Connect Puppeteer                                       │
│    • debuggerUrl = "ws://localhost:54321/devtools/browser"       │
│    • cmd = exec.Command("node", "puppeteer.js", debuggerUrl)     │
│    • stdin, stdout = cmd.StdinPipe(), cmd.StdoutPipe()           │
│    • cmd.Start()                                                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (9) Puppeteer process started
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                    Puppeteer.js (Node.js)                        │
│  • puppeteer.connect({browserWSEndpoint: debuggerUrl})           │
│  • browser.newPage()                                             │
│  • Listen on stdin for commands                                  │
│  • Write "ready" to stdout                                       │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (10) {"type": "ready"}
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                       Session Manager                            │
│  Step 4: Create session object                                   │
│    • Generate UUID                                               │
│    • Store in sync.Map                                           │
│    • Start timeout goroutine                                     │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (11) Session created
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                    CreateSession Handler                         │
│  • Build JSON response                                           │
│  • Return 200 OK                                                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ (12) HTTP 200 OK
                            │
┌───────────────────────────┴─────────────────────────────────────┐
│                         Client Response                          │
│  {                                                               │
│    "id": "sess_abc123",                                          │
│    "status": "RUNNING",                                          │
│    "projectId": "my-app",                                        │
│    "region": "us-west-2",                                        │
│    "debuggerUrl": "ws://localhost:54321/devtools/browser",       │
│    "createdAt": "2025-12-09T10:00:00Z",                          │
│    "expiresAt": "2025-12-09T11:00:00Z"                           │
│  }                                                               │
└─────────────────────────────────────────────────────────────────┘
```

This architecture documentation provides a deep dive into the system design and implementation details of Browserbase Mini.
