package server

import (
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"pptx-remote/internal/crypto"
	"pptx-remote/internal/ppt"
)

func setupTestServer(t *testing.T) (*httptest.Server, *Server, *SessionManager, *ppt.MockController) {
	mockPPT := ppt.NewMockController()
	sessionMgr, err := NewSessionManager(nil, nil)
	if err != nil {
		t.Fatalf("NewSessionManager failed: %v", err)
	}

	remoteHTML := []byte("<html><body>Mobile Remote App</body></html>")
	pairingHTML := []byte("<html><body>Pairing Dashboard</body></html>")

	srv := NewServer(0, "http://localhost", mockPPT, sessionMgr, remoteHTML, pairingHTML, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleRemoteApp)
	mux.HandleFunc("/pairing", srv.handlePairingPage)
	mux.HandleFunc("/pairing/qr.png", srv.handleQRCodePNG)
	mux.HandleFunc("/api/info", srv.handleAPIInfo)
	mux.HandleFunc("/ws", srv.handleWebSocket)

	ts := httptest.NewServer(mux)
	srv.baseURL = ts.URL

	return ts, srv, sessionMgr, mockPPT
}

func TestHTTPServing(t *testing.T) {
	ts, _, _, _ := setupTestServer(t)
	defer ts.Close()

	// 1. Root remote page
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// 2. Pairing dashboard
	resp, err = http.Get(ts.URL + "/pairing")
	if err != nil {
		t.Fatalf("GET /pairing failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// 3. QR PNG
	resp, err = http.Get(ts.URL + "/pairing/qr.png")
	if err != nil {
		t.Fatalf("GET /pairing/qr.png failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if _, err := png.Decode(resp.Body); err != nil {
		t.Errorf("expected valid PNG image, got: %v", err)
	}

	// 4. API Info
	resp, err = http.Get(ts.URL + "/api/info")
	if err != nil {
		t.Fatalf("GET /api/info failed: %v", err)
	}
	defer resp.Body.Close()
	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode JSON from /api/info: %v", err)
	}
	if info["pin"] == nil || len(info["pin"].(string)) != 4 {
		t.Errorf("invalid or missing PIN in /api/info: %v", info["pin"])
	}
}

func TestWebSocketE2EEPairingAndControl(t *testing.T) {
	ts, _, sessionMgr, mockPPT := setupTestServer(t)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer ws.Close()

	masterKey := sessionMgr.GetMasterKey()
	pairingToken, _, pin := sessionMgr.PairingInfo()

	// 1. Test invalid PIN verification
	badPairReq := PairRequestMessage{
		Type:         MsgTypePairRequest,
		PairingToken: pairingToken,
		PIN:          "9999", // Intentional wrong PIN (unless pin is 9999)
	}
	if pin == "9999" {
		badPairReq.PIN = "0000"
	}

	payload, _ := json.Marshal(badPairReq)
	iv, data, err := crypto.Encrypt(masterKey, payload)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	envBytes, _ := json.Marshal(WireEnvelope{IV: iv, Data: data})
	if err := ws.WriteMessage(websocket.TextMessage, envBytes); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}

	// Read response
	_, respBytes, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage error: %v", err)
	}

	var respEnv WireEnvelope
	_ = json.Unmarshal(respBytes, &respEnv)
	decrypted, err := crypto.Decrypt(masterKey, respEnv.IV, respEnv.Data)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}

	var pairResp PairResponseMessage
	_ = json.Unmarshal(decrypted, &pairResp)
	if pairResp.Success {
		t.Fatal("expected pairing failure with wrong PIN, got success")
	}

	// 2. Test valid PIN verification
	goodPairReq := PairRequestMessage{
		Type:         MsgTypePairRequest,
		PairingToken: pairingToken,
		PIN:          pin,
		DeviceName:   "Test Runner iPhone",
	}

	payload, _ = json.Marshal(goodPairReq)
	iv, data, err = crypto.Encrypt(masterKey, payload)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	envBytes, _ = json.Marshal(WireEnvelope{IV: iv, Data: data})
	if err := ws.WriteMessage(websocket.TextMessage, envBytes); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}

	// Read valid pair response
	_, respBytes, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage error: %v", err)
	}

	_ = json.Unmarshal(respBytes, &respEnv)
	decrypted, err = crypto.Decrypt(masterKey, respEnv.IV, respEnv.Data)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}

	_ = json.Unmarshal(decrypted, &pairResp)
	if !pairResp.Success {
		t.Fatalf("expected pairing success with correct PIN, got error: %s", pairResp.Error)
	}
	if pairResp.SessionID == "" {
		t.Fatal("expected non-empty SessionID on successful pairing")
	}
	if pairResp.State == nil || pairResp.State.CurrentSlide != 1 {
		t.Fatalf("expected state with slide 1, got: %+v", pairResp.State)
	}

	storedSessionID := pairResp.SessionID

	// 3. Test sending ACTION (NEXT_SLIDE)
	actionReq := ActionMessage{
		Type:   MsgTypeAction,
		Action: ppt.ActionNextSlide,
	}
	payload, _ = json.Marshal(actionReq)
	iv, data, _ = crypto.Encrypt(masterKey, payload)
	envBytes, _ = json.Marshal(WireEnvelope{IV: iv, Data: data})

	if err := ws.WriteMessage(websocket.TextMessage, envBytes); err != nil {
		t.Fatalf("Write action error: %v", err)
	}

	// Read broadcasted STATE_UPDATE
	_, respBytes, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("Read action response error: %v", err)
	}

	_ = json.Unmarshal(respBytes, &respEnv)
	decrypted, err = crypto.Decrypt(masterKey, respEnv.IV, respEnv.Data)
	if err != nil {
		t.Fatalf("Decrypt state update error: %v", err)
	}

	var stateUpdate StateUpdateMessage
	_ = json.Unmarshal(decrypted, &stateUpdate)
	if stateUpdate.State.CurrentSlide != 2 {
		t.Errorf("expected current slide 2 after NEXT_SLIDE, got %d", stateUpdate.State.CurrentSlide)
	}

	// 4. Test reconnect with RESUME_REQUEST
	_ = ws.Close()

	// New connection to simulate phone waking up
	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Second dial failed: %v", err)
	}
	defer ws2.Close()

	resumeReq := ResumeRequestMessage{
		Type:      MsgTypeResumeRequest,
		SessionID: storedSessionID,
	}
	payload, _ = json.Marshal(resumeReq)
	iv, data, _ = crypto.Encrypt(masterKey, payload)
	envBytes, _ = json.Marshal(WireEnvelope{IV: iv, Data: data})

	if err := ws2.WriteMessage(websocket.TextMessage, envBytes); err != nil {
		t.Fatalf("Write resume error: %v", err)
	}

	_, respBytes, err = ws2.ReadMessage()
	if err != nil {
		t.Fatalf("Read resume response error: %v", err)
	}

	_ = json.Unmarshal(respBytes, &respEnv)
	decrypted, err = crypto.Decrypt(masterKey, respEnv.IV, respEnv.Data)
	if err != nil {
		t.Fatalf("Decrypt resume response error: %v", err)
	}

	var resumeResp PairResponseMessage
	_ = json.Unmarshal(decrypted, &resumeResp)
	if !resumeResp.Success {
		t.Fatalf("expected successful resume, got error: %s", resumeResp.Error)
	}
	if resumeResp.State.CurrentSlide != 2 {
		t.Errorf("expected preserved slide 2 on resume, got %d", resumeResp.State.CurrentSlide)
	}

	// Confirm mockPPT state is in sync
	curState, _ := mockPPT.GetState(context.Background())
	if curState.CurrentSlide != 2 {
		t.Errorf("mockPPT state mismatch: expected 2, got %d", curState.CurrentSlide)
	}
}

func TestPinBruteForceLockout(t *testing.T) {
	sm, err := NewSessionManager(nil, nil)
	if err != nil {
		t.Fatalf("NewSessionManager error: %v", err)
	}

	token, _, pin := sm.PairingInfo()
	wrongPin := "1111"
	if pin == "1111" {
		wrongPin = "2222"
	}

	// 5 failed attempts
	for i := 0; i < MaxFailedPinAttempts; i++ {
		_, err := sm.VerifyPairing(token, wrongPin, "Attacker")
		if err != ErrInvalidPin {
			t.Fatalf("attempt %d: expected ErrInvalidPin, got %v", i+1, err)
		}
	}

	// 6th attempt should trigger ErrSessionLockedOut
	_, err = sm.VerifyPairing(token, wrongPin, "Attacker")
	if !strings.Contains(err.Error(), "too many failed PIN attempts") {
		t.Fatalf("expected lockout error, got: %v", err)
	}

	// Even correct PIN should be rejected while locked out
	_, err = sm.VerifyPairing(token, pin, "Presenter")
	if !strings.Contains(err.Error(), "too many failed PIN attempts") {
		t.Fatalf("expected lockout rejection even with correct pin during cooldown, got: %v", err)
	}
}
