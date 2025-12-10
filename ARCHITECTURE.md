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
