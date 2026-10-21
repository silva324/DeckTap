package server

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"pptx-remote/internal/crypto"
)

var (
	ErrInvalidPairingToken = errors.New("invalid pairing token")
	ErrInvalidPin          = errors.New("invalid PIN verification code")
	ErrSessionLockedOut    = errors.New("too many failed PIN attempts; please wait 30 seconds")
	ErrInvalidSessionID    = errors.New("invalid or expired session ID")
)

const (
	MaxFailedPinAttempts = 5
	PinLockoutDuration   = 30 * time.Second
)

// SessionManager manages cryptographic material, active sessions, and PIN verification.
type SessionManager struct {
	mu             sync.RWMutex
	masterKey      []byte
	masterKeyStr   string
	pairingToken   string
	pinCode        string
	sessionID      string
	isPaired       bool
	clientDevice   string
	failedAttempts int
	lockoutUntil   time.Time
	logger         *log.Logger
	onStatusChange func(status string, isPaired bool)
}

func NewSessionManager(logger *log.Logger, onStatusChange func(status string, isPaired bool)) (*SessionManager, error) {
	if logger == nil {
		logger = log.Default()
	}
	sm := &SessionManager{
		logger:         logger,
		onStatusChange: onStatusChange,
	}
	if err := sm.RotateKeys(); err != nil {
		return nil, err
	}
	return sm, nil
}

// RotateKeys regenerates all cryptographic secrets and invalidates any existing paired sessions.
func (sm *SessionManager) RotateKeys() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	rawKey, keyStr, err := crypto.GenerateMasterKey()
	if err != nil {
		return fmt.Errorf("failed to generate master key: %w", err)
	}

	_, tokenStr, err := crypto.GeneratePairingToken()
	if err != nil {
		return fmt.Errorf("failed to generate pairing token: %w", err)
	}

	pin, err := crypto.GeneratePinCode()
	if err != nil {
		return fmt.Errorf("failed to generate pin code: %w", err)
	}

	sm.masterKey = rawKey
	sm.masterKeyStr = keyStr
	sm.pairingToken = tokenStr
	sm.pinCode = pin
	sm.sessionID = ""
	sm.isPaired = false
	sm.clientDevice = ""
	sm.failedAttempts = 0
	sm.lockoutUntil = time.Time{}

	sm.logger.Printf("[Security] Keys rotated. New PIN: %s | Pairing Token: %s...", sm.pinCode, sm.pairingToken[:8])
	if sm.onStatusChange != nil {
		sm.onStatusChange(fmt.Sprintf("Ready (PIN: %s)", sm.pinCode), false)
	}

	return nil
}

// GetMasterKey returns the raw AES key for encryption/decryption.
func (sm *SessionManager) GetMasterKey() []byte {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	keyCopy := make([]byte, len(sm.masterKey))
	copy(keyCopy, sm.masterKey)
	return keyCopy
}

// PairingInfo returns parameters needed for QR code and URI generation.
func (sm *SessionManager) PairingInfo() (token string, masterKeyStr string, pin string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.pairingToken, sm.masterKeyStr, sm.pinCode
}

// Status returns current pairing information.
func (sm *SessionManager) Status() (isPaired bool, deviceName string, pin string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isPaired, sm.clientDevice, sm.pinCode
}

// VerifyPairing validates the one-time pairing token and 4-digit PIN under rate-limiting.
func (sm *SessionManager) VerifyPairing(token string, pin string, deviceName string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	if now.Before(sm.lockoutUntil) {
		remaining := int(sm.lockoutUntil.Sub(now).Seconds())
		return "", fmt.Errorf("%w (retry in %ds)", ErrSessionLockedOut, remaining)
	}

	if !crypto.ConstantTimeCompare(token, sm.pairingToken) {
		return "", ErrInvalidPairingToken
	}

	if !crypto.ConstantTimeCompare(pin, sm.pinCode) {
		sm.failedAttempts++
		sm.logger.Printf("[Security] Invalid PIN entered (Attempt %d/%d)", sm.failedAttempts, MaxFailedPinAttempts)
		if sm.failedAttempts >= MaxFailedPinAttempts {
			sm.lockoutUntil = now.Add(PinLockoutDuration)
			sm.logger.Printf("[Security] Rate-limit triggered: locked out for %v", PinLockoutDuration)
		}
		return "", ErrInvalidPin
	}

	// Successful verification: generate session ID
	_, sessionIDStr, err := crypto.GenerateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	sm.sessionID = sessionIDStr
	sm.isPaired = true
	sm.clientDevice = deviceName
	sm.failedAttempts = 0
	sm.lockoutUntil = time.Time{}

	sm.logger.Printf("[Security] Pairing SUCCESS from %q (Session ID: %s...)", deviceName, sessionIDStr[:8])
	if sm.onStatusChange != nil {
		displayDevice := deviceName
		if displayDevice == "" {
			displayDevice = "Mobile Device"
		}
		sm.onStatusChange(fmt.Sprintf("Connected (%s)", displayDevice), true)
	}

	return sessionIDStr, nil
}

// ValidateResume validates a reconnect attempt using an existing SessionID.
func (sm *SessionManager) ValidateResume(sessionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.sessionID == "" || sessionID == "" {
		return false
	}
	return crypto.ConstantTimeCompare(sessionID, sm.sessionID)
}

// Unpair marks the session as disconnected.
func (sm *SessionManager) Unpair() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.isPaired = false
	sm.logger.Printf("[Security] Session disconnected")
	if sm.onStatusChange != nil {
		sm.onStatusChange(fmt.Sprintf("Ready (PIN: %s)", sm.pinCode), false)
	}
}
