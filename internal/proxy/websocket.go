package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shehryarbajwa/browserbase-mini/internal/session"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	sessionMgr *session.Manager
}

func NewServer(sessionMgr *session.Manager) *Server {
	return &Server{
		sessionMgr: sessionMgr,
	}
}

func (s *Server) HandleDebugConnection(w http.ResponseWriter, r *http.Request, sessionID string) {
	// Get session
	sess, err := s.sessionMgr.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if sess.Status != "RUNNING" {
		http.Error(w, "Session is not running", http.StatusBadRequest)
		return
	}

	// Upgrade HTTP connection to WebSocket
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer clientConn.Close()

	log.Printf("✅ Client connected to session %s debug", sessionID)

	// For browserless/chrome, connect to root WebSocket (not /devtools/page/...)
	chromeURL := sess.ConnectURL

	log.Printf("Connecting to Chrome at %s", chromeURL)

	// Connect to Chrome CDP WebSocket
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chromeConn, _, err := websocket.DefaultDialer.DialContext(ctx, chromeURL, nil)
	if err != nil {
		log.Printf("❌ Failed to connect to Chrome: %v", err)
		clientConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error connecting: %v", err)))
		return
	}
	defer chromeConn.Close()

	log.Printf("✅ Connected to Chrome for session %s", sessionID)

	// Bidirectional proxy
	errChan := make(chan error, 2)

	// Client → Chrome
	go func() {
		errChan <- s.proxyMessages(clientConn, chromeConn, "client→chrome")
	}()

	// Chrome → Client
	go func() {
		errChan <- s.proxyMessages(chromeConn, clientConn, "chrome→client")
	}()

	// Wait for either direction to close
	err = <-errChan
	if err != nil && err != io.EOF {
		log.Printf("Proxy error for session %s: %v", sessionID, err)
	}

	log.Printf("Client disconnected from session %s debug", sessionID)
}

func (s *Server) proxyMessages(src, dst *websocket.Conn, direction string) error {
	for {
		messageType, message, err := src.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error (%s): %v", direction, err)
			}
			return err
		}

		if err := dst.WriteMessage(messageType, message); err != nil {
			log.Printf("Failed to write message (%s): %v", direction, err)
			return err
		}
	}
}
