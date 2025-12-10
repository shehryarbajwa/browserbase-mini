package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/shehryarbajwa/browserbase-mini/internal/api"
	contextmgr "github.com/shehryarbajwa/browserbase-mini/internal/context"
	"github.com/shehryarbajwa/browserbase-mini/internal/proxy"
	"github.com/shehryarbajwa/browserbase-mini/internal/ratelimit"
	"github.com/shehryarbajwa/browserbase-mini/internal/region"
	"github.com/shehryarbajwa/browserbase-mini/internal/session"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	log.Println("Starting Browserbase Mini...")

	// Initialize region manager
	regionMgr, err := region.NewManager()
	if err != nil {
		log.Fatalf("Failed to create region manager: %v", err)
	}
	defer regionMgr.Close()
	log.Println("✓ Region manager initialized (3 regions)")

	// Ensure Chrome image is available
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Println("⏳ Ensuring Chrome images are available...")
	if err := regionMgr.EnsureImages(ctx); err != nil {
		log.Fatalf("Failed to ensure images: %v", err)
	}
	log.Println("✓ Chrome images ready in all regions")

	// Initialize context manager
	ctxMgr, err := contextmgr.NewManager("./storage/contexts")
	if err != nil {
		log.Fatalf("Failed to create context manager: %v", err)
	}
	log.Println("✓ Context manager initialized")

	// Initialize session manager
	sessionMgr := session.NewManager(regionMgr, ctxMgr)
	log.Println("✓ Session manager initialized")

	// Initialize WebSocket proxy
	proxyServer := proxy.NewServer(sessionMgr)
	log.Println("✓ WebSocket proxy initialized")

	// Initialize rate limiter (100 requests/hour, burst of 10)
	rateLimiter := ratelimit.NewLimiter(100, 10)
	log.Println("✓ Rate limiter initialized (100 req/hour per project)")

	// Setup HTTP handlers
	sessionHandler := api.NewHandler(sessionMgr)
	contextHandler := api.NewContextHandler(ctxMgr)

	router := sessionHandler.SetupRoutes(contextHandler, proxyServer, rateLimiter)
	log.Println("✓ HTTP routes configured")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	go func() {
		log.Println("🚀 Server starting on http://localhost:8080")
		log.Println("📍 API endpoints available at http://localhost:8080/v1")
		log.Println("🌍 Regions: us-west-2, us-east-1, eu-central-1")
		log.Println("💾 Contexts: Create, load, and persist browser state")
		log.Println("🔍 Debug: Live WebSocket proxy for CDP debugging")
		log.Println("⏱️  Rate Limit: 100 requests/hour per project")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("\n⏳ Shutting down server gracefully...")

	// Shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server stopped cleanly")
}
