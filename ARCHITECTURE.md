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

Browserbase Mini is a browser automation platform built around containerized Chrome instances. The system follows a layered architecture:

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
