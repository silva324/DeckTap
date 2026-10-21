package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	qrcode "github.com/skip2/go-qrcode"

	"pptx-remote/internal/crypto"
	"pptx-remote/internal/ppt"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow WebSocket connections from local network, localhost, or Cloudflare tunnel
		return true
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// Client represents an active WebSocket connection.
type Client struct {
	conn          *websocket.Conn
	authenticated bool
	deviceName    string
	sendChan      chan []byte
}

// Server is the HTTP and WebSocket engine for Zero-Trust Remote.
type Server struct {
	httpServer     *http.Server
	pptCtrl        ppt.PowerPointController
	sessionMgr     *SessionManager
	baseURL        string
	logger         *log.Logger
	clientsMu      sync.RWMutex
	clients        map[*Client]bool
	lastState      ppt.PresentationState
	lastStateMu    sync.RWMutex
	remoteHTML     []byte
	pairingHTML    []byte
	onClientChange func(count int)
}

func NewServer(
	port int,
	baseURL string,
	pptCtrl ppt.PowerPointController,
	sessionMgr *SessionManager,
	remoteHTML []byte,
	pairingHTML []byte,
	logger *log.Logger,
	onClientChange func(count int),
) *Server {
	if logger == nil {
		logger = log.Default()
	}

	initState, _ := pptCtrl.GetState(context.Background())

	s := &Server{
		pptCtrl:        pptCtrl,
		sessionMgr:     sessionMgr,
		baseURL:        baseURL,
		remoteHTML:     remoteHTML,
		pairingHTML:    pairingHTML,
		logger:         logger,
		clients:        make(map[*Client]bool),
		lastState:      initState,
		onClientChange: onClientChange,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRemoteApp)
	mux.HandleFunc("/pairing", s.handlePairingPage)
	mux.HandleFunc("/pairing/qr.png", s.handleQRCodePNG)
	mux.HandleFunc("/api/info", s.handleAPIInfo)
	mux.HandleFunc("/ws", s.handleWebSocket)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return s
}

// SetBaseURL updates the base URL (e.g. after cloudflared tunnel is established).
func (s *Server) SetBaseURL(url string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	s.baseURL = url
}

// GetPairingURI constructs the full pairing URI with out-of-band fragment secrets.
func (s *Server) GetPairingURI() string {
	token, key, _ := s.sessionMgr.PairingInfo()
	return fmt.Sprintf("%s/#%s.%s", s.baseURL, token, key)
}

// Start begins listening and starts the background state synchronization loop.
func (s *Server) Start(ctx context.Context) error {
	go s.runStatePollingLoop(ctx)

	s.logger.Printf("[Server] Listening on http://%s (Base URL: %s)", s.httpServer.Addr, s.baseURL)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown cleanly stops the server and closes all active client connections.
func (s *Server) Shutdown(ctx context.Context) error {
	s.clientsMu.Lock()
	for client := range s.clients {
		_ = client.conn.Close()
	}
	s.clients = make(map[*Client]bool)
	s.clientsMu.Unlock()

	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleRemoteApp(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write(s.remoteHTML)
}

func (s *Server) handlePairingPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write(s.pairingHTML)
}

func (s *Server) handleQRCodePNG(w http.ResponseWriter, r *http.Request) {
	uri := s.GetPairingURI()
	png, err := qrcode.Encode(uri, qrcode.Medium, 380)
	if err != nil {
		http.Error(w, "Failed to render QR code", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write(png)
}

func (s *Server) handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	isPaired, device, pin := s.sessionMgr.Status()
	token, key, _ := s.sessionMgr.PairingInfo()

	resp := map[string]interface{}{
		"base_url":      s.baseURL,
		"pairing_uri":   fmt.Sprintf("%s/#%s.%s", s.baseURL, token, key),
		"pin":           pin,
		"is_paired":     isPaired,
		"device_name":   device,
		"ppt_engine":    s.pptCtrl.Name(),
		"active_remote": len(s.clients),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("[WS] Upgrade failed: %v", err)
		return
	}

	client := &Client{
		conn:          conn,
		authenticated: false,
		sendChan:      make(chan []byte, 64),
	}

	s.clientsMu.Lock()
	s.clients[client] = true
	count := len(s.clients)
	s.clientsMu.Unlock()

	if s.onClientChange != nil {
		s.onClientChange(count)
	}

	go s.clientWriter(client)
	s.clientReader(client)
}

func (s *Server) clientWriter(client *Client) {
	defer func() {
		_ = client.conn.Close()
	}()

	for msg := range client.sendChan {
		_ = client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (s *Server) clientReader(client *Client) {
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, client)
		count := len(s.clients)
		s.clientsMu.Unlock()

		close(client.sendChan)
		_ = client.conn.Close()

		if client.authenticated {
			s.sessionMgr.Unpair()
		}

		if s.onClientChange != nil {
			s.onClientChange(count)
		}
	}()

	client.conn.SetReadLimit(65536)
	_ = client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		_ = client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, rawMsg, err := client.conn.ReadMessage()
		if err != nil {
			break
		}

		// Reset deadline on incoming message
		_ = client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Parse WireEnvelope
		var env WireEnvelope
		if err := json.Unmarshal(rawMsg, &env); err != nil {
			s.logger.Printf("[WS] Malformed wire envelope: %v", err)
			continue
		}

		// Decrypt payload using MasterKey
		key := s.sessionMgr.GetMasterKey()
		plaintext, err := crypto.Decrypt(key, env.IV, env.Data)
		if err != nil {
			s.logger.Printf("[Security] Decryption failed: %v", err)
			s.sendEncrypted(client, AlertMessage{
				Type:    MsgTypeAlert,
				Message: "Decryption failed. Please refresh and re-scan QR code.",
			})
			continue
		}

		var base BaseMessage
		if err := json.Unmarshal(plaintext, &base); err != nil {
			s.logger.Printf("[WS] Malformed plaintext JSON: %v", err)
			continue
		}

		s.processClientMessage(client, base.Type, plaintext)
	}
}

func (s *Server) processClientMessage(client *Client, msgType MessageType, raw []byte) {
	ctx := context.Background()

	switch msgType {
	case MsgTypePairRequest:
		var req PairRequestMessage
		if err := json.Unmarshal(raw, &req); err != nil {
			return
		}

		sessionID, err := s.sessionMgr.VerifyPairing(req.PairingToken, req.PIN, req.DeviceName)
		if err != nil {
			s.sendEncrypted(client, PairResponseMessage{
				Type:    MsgTypePairResponse,
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		client.authenticated = true
		client.deviceName = req.DeviceName

		s.lastStateMu.RLock()
		state := s.lastState
		s.lastStateMu.RUnlock()

		if state.TotalSlides == 0 {
			state, _ = s.pptCtrl.GetState(ctx)
			s.lastStateMu.Lock()
			s.lastState = state
			s.lastStateMu.Unlock()
		}

		s.sendEncrypted(client, PairResponseMessage{
			Type:      MsgTypePairResponse,
			Success:   true,
			SessionID: sessionID,
			State:     &state,
		})
		s.logger.Printf("[WS] Dispatched PAIR_RESPONSE to %q (Slide %d/%d)", client.deviceName, state.CurrentSlide, state.TotalSlides)

		go func() {
			newState, err := s.pptCtrl.GetState(context.Background())
			if err == nil && (newState.CurrentSlide != state.CurrentSlide || newState.Title != state.Title) {
				s.lastStateMu.Lock()
				s.lastState = newState
				s.lastStateMu.Unlock()
				s.BroadcastState(newState)
			}
		}()

	case MsgTypeResumeRequest:
		var req ResumeRequestMessage
		if err := json.Unmarshal(raw, &req); err != nil {
			return
		}

		if !s.sessionMgr.ValidateResume(req.SessionID) {
			s.sendEncrypted(client, PairResponseMessage{
				Type:    MsgTypeResumeResponse,
				Success: false,
				Error:   "Session expired or invalid. Please verify PIN again.",
			})
			return
		}

		client.authenticated = true
		s.lastStateMu.RLock()
		state := s.lastState
		s.lastStateMu.RUnlock()

		s.sendEncrypted(client, PairResponseMessage{
			Type:      MsgTypeResumeResponse,
			Success:   true,
			SessionID: req.SessionID,
			State:     &state,
		})
		s.logger.Printf("[WS] Dispatched RESUME_RESPONSE to %q", client.deviceName)

	case MsgTypeAction:
		if !client.authenticated {
			s.sendEncrypted(client, AlertMessage{
				Type:    MsgTypeAlert,
				Message: "Unauthorized: PIN verification required.",
			})
			return
		}

		var actionMsg ActionMessage
		if err := json.Unmarshal(raw, &actionMsg); err != nil {
			return
		}

		s.logger.Printf("[Action] Executing %s (Target: %d) from %s", actionMsg.Action, actionMsg.TargetSlide, client.deviceName)
		newState, err := s.pptCtrl.ExecuteAction(ctx, actionMsg.Action, actionMsg.TargetSlide)
		if err != nil {
			s.logger.Printf("[Action] Controller error: %v", err)
		}

		s.lastStateMu.Lock()
		s.lastState = newState
		s.lastStateMu.Unlock()

		s.BroadcastState(newState)

	case MsgTypePing:
		var ping PingMessage
		_ = json.Unmarshal(raw, &ping)
		s.sendEncrypted(client, PongMessage{
			Type:      MsgTypePong,
			Timestamp: ping.Timestamp,
		})
	}
}

func (s *Server) sendEncrypted(client *Client, v interface{}) {
	key := s.sessionMgr.GetMasterKey()
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}

	iv, data, err := crypto.Encrypt(key, payload)
	if err != nil {
		s.logger.Printf("[Crypto] Failed to encrypt outgoing message: %v", err)
		return
	}

	envBytes, err := json.Marshal(WireEnvelope{
		IV:   iv,
		Data: data,
	})
	if err != nil {
		return
	}

	select {
	case client.sendChan <- envBytes:
	default:
		// Drop message if channel is clogged to prevent blocking
	}
}

// BroadcastState sends presentation state updates to all authenticated remotes.
func (s *Server) BroadcastState(state ppt.PresentationState) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for client := range s.clients {
		if client.authenticated {
			s.sendEncrypted(client, StateUpdateMessage{
				Type:  MsgTypeStateUpdate,
				State: state,
			})
		}
	}
}

// BroadcastAlert broadcasts a security or system alert to all connected remotes.
func (s *Server) BroadcastAlert(msg string) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for client := range s.clients {
		s.sendEncrypted(client, AlertMessage{
			Type:    MsgTypeAlert,
			Message: msg,
		})
	}
}

func (s *Server) runStatePollingLoop(ctx context.Context) {
	ticker := time.NewTicker(600 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.clientsMu.RLock()
			hasAuthClient := false
			for client := range s.clients {
				if client.authenticated {
					hasAuthClient = true
					break
				}
			}
			s.clientsMu.RUnlock()

			if !hasAuthClient {
				continue
			}

			state, err := s.pptCtrl.GetState(ctx)
			if err != nil {
				continue
			}

			s.lastStateMu.Lock()
			hasChanged := state.CurrentSlide != s.lastState.CurrentSlide ||
				state.Running != s.lastState.Running ||
				state.Available != s.lastState.Available ||
				state.BlackScreen != s.lastState.BlackScreen ||
				state.WhiteScreen != s.lastState.WhiteScreen ||
				state.TotalSlides != s.lastState.TotalSlides
			if hasChanged {
				s.lastState = state
			}
			s.lastStateMu.Unlock()

			if hasChanged {
				s.BroadcastState(state)
			}
		}
	}
}
