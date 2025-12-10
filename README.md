# Browserbase Mini

A lightweight, containerized browser automation service that mimics the Browserbase API. Built with Go, Docker, and React for managing ephemeral browser sessions with persistent context support.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Getting Started](#getting-started)
- [API Documentation](#api-documentation)
- [Frontend Dashboard](#frontend-dashboard)
- [Project Structure](#project-structure)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

## Overview

Browserbase Mini provides a complete browser automation platform with:
- **Multi-region browser pools** (us-west-2, us-east-1, eu-central-1)
- **Persistent browser contexts** using tar.gz compression
- **Live debugging** via Chrome DevTools Protocol (CDP)
- **Screenshot capture** and page navigation
- **Rate limiting** (100 requests/hour per project)
- **React dashboard** for session management

## Features

- **Browser Session Management**: Create, list, and delete browser sessions
- **Context Persistence**: Save and restore browser state (cookies, localStorage, etc.)
- **Multi-Region Support**: Deploy browsers across multiple regions
- **Live Screenshots**: Capture webpage screenshots as base64 PNG
- **WebSocket Debugging**: Full CDP protocol access for debugging
- **Rate Limiting**: Token bucket algorithm for API protection
- **Concurrency Control**: 10 concurrent sessions per project
- **Docker Containerization**: Isolated browser instances using browserless/chrome
- **React UI**: User-friendly dashboard for session visualization

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       Client Request                         │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   API Server (Port 8080)                     │
│                    Gorilla Mux Router                        │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│               Rate Limit Middleware                          │
│          (100 req/hour per projectId)                        │
└───────────────────────────┬─────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│   Session    │   │   Context    │   │   Region     │
│   Manager    │   │   Manager    │   │   Manager    │
└──────┬───────┘   └──────┬───────┘   └──────┬───────┘
       │                  │                   │
       │                  │                   │
       ▼                  ▼                   ▼
┌──────────────────────────────────────────────────────┐
│              Browser Pool (Docker SDK)                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │
│  │ us-west-2   │  │ us-east-1   │  │ eu-central-1│  │
│  │ :9222       │  │ :9322       │  │ :9422       │  │
│  └─────────────┘  └─────────────┘  └─────────────┘  │
└──────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│           Puppeteer Bridge (Node.js)                  │
│  ┌──────────────────────────────────────────────┐    │
│  │  • Navigate to URLs                          │    │
│  │  • Capture screenshots                       │    │
│  │  • Execute JavaScript                        │    │
│  │  • Manage browser lifecycle                  │    │
│  └──────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│        Docker Containers (browserless/chrome)         │
│  ┌────────────────────────────────────────────────┐  │
│  │  Chrome Headless Browser Instances             │  │
│  │  • Isolated environment                        │  │
│  │  • Mounted /data volumes                       │  │
│  │  • Auto-cleanup on timeout                     │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│            Context Storage (./storage/contexts/)      │
│  • Tar.gz compressed browser state                    │
│  • Cookies, localStorage, sessionStorage              │
│  • Reusable across sessions                           │
└──────────────────────────────────────────────────────┘
```

See [ARCHITECTURE.md](./ARCHITECTURE.md) for detailed architecture documentation.

## Prerequisites

Before installing Browserbase Mini, ensure you have:

### Required

- **Go 1.25.3 or higher** - [Download Go](https://golang.org/dl/)
- **Node.js 16.x or higher** - [Download Node.js](https://nodejs.org/)
- **Docker Desktop** - [Download Docker](https://www.docker.com/products/docker-desktop/)
- **Git** - For cloning the repository

### System Requirements

- **OS**: macOS, Linux, or Windows (with WSL2 for Docker)
- **RAM**: Minimum 4GB (8GB+ recommended for multiple sessions)
- **Disk Space**: 5GB+ (for Docker images and browser contexts)
- **Docker Socket Access**: Docker daemon must be running

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/shehryarbajwa/browserbase-mini.git
cd browserbase-mini
```

### 2. Install Go Dependencies

```bash
go mod download
```

### 3. Install Node.js Dependencies (Backend Puppeteer)

```bash
npm install
```

### 4. Install Frontend Dependencies

```bash
cd frontend
npm install
cd ..
```

### 5. Verify Docker Installation

```bash
docker --version
docker ps  # Should list running containers without errors
```

## Configuration

### Environment Variables

Create a `.env` file in the project root:

```bash
cp .env.example .env
```

Edit `.env` with your configuration:

```env
# Docker Configuration
# Path to Docker socket (auto-detected on most systems)
DOCKER_HOST=unix:///var/run/docker.sock

# For macOS Docker Desktop users:
# DOCKER_HOST=unix:///Users/<your-username>/.docker/run/docker.sock

# For Windows WSL2:
# DOCKER_HOST=unix:///var/run/docker-desktop.sock
```

### Docker Socket Path Detection

To find your Docker socket path:

**macOS:**
```bash
ls -la ~/.docker/run/docker.sock
# Usually: unix:///Users/<username>/.docker/run/docker.sock
```

**Linux:**
```bash
ls -la /var/run/docker.sock
# Usually: unix:///var/run/docker.sock
```

**Windows (WSL2):**
```bash
ls -la /var/run/docker-desktop.sock
# Usually: unix:///var/run/docker-desktop.sock
```

### Configuration Constants

The following constants are hardcoded in the application (can be modified in source):

| Setting | Default | Location |
|---------|---------|----------|
| API Port | `8080` | `cmd/server/main.go` |
| Frontend Dev Port | `5173` | `frontend/vite.config.js` |
| Session Timeout | `3600s` (1 hour) | `internal/api/handlers.go` |
| Max Sessions/Project | `10` | `internal/session/manager.go` |
| Rate Limit | `100 req/hour` | `cmd/server/main.go` |
| Rate Limit Burst | `10` | `cmd/server/main.go` |

## Getting Started

### Option 1: Run Backend + Frontend Separately (Development)

**Terminal 1 - Start Backend:**
```bash
go run cmd/server/main.go
```

**Terminal 2 - Start Frontend:**
```bash
cd frontend
npm run dev
```

Access the application:
- Frontend Dashboard: http://localhost:5173
- Backend API: http://localhost:8080

### Option 2: Build and Run Production

**Build Backend:**
```bash
go build -o browserbase-mini cmd/server/main.go
```

**Build Frontend:**
```bash
cd frontend
npm run build
cd ..
```

**Run Backend:**
```bash
./browserbase-mini
```

**Serve Frontend:**
```bash
# Option 1: Use a static file server
npx serve -s frontend/dist -p 5173

# Option 2: Use Python
cd frontend/dist
python3 -m http.server 5173
```
