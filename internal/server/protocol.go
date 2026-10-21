package server

import "pptx-remote/internal/ppt"

// WireEnvelope represents the E2EE AES-256-GCM container sent over WebSocket.
type WireEnvelope struct {
	IV   string `json:"iv"`   // Base64URL 12-byte IV
	Data string `json:"data"` // Base64URL ciphertext + 16-byte auth tag
}

// MessageType represents the encrypted payload command type.
type MessageType string

const (
	MsgTypePairRequest    MessageType = "PAIR_REQUEST"
	MsgTypePairResponse   MessageType = "PAIR_RESPONSE"
	MsgTypeResumeRequest  MessageType = "RESUME_REQUEST"
	MsgTypeResumeResponse MessageType = "RESUME_RESPONSE"
	MsgTypeAction         MessageType = "ACTION"
	MsgTypeStateUpdate    MessageType = "STATE_UPDATE"
	MsgTypePing           MessageType = "PING"
	MsgTypePong           MessageType = "PONG"
	MsgTypeError          MessageType = "ERROR"
	MsgTypeAlert          MessageType = "ALERT"
)

// BaseMessage contains the discriminator for encrypted messages.
type BaseMessage struct {
	Type MessageType `json:"type"`
}

// PairRequestMessage is sent by the mobile client to verify the PIN.
type PairRequestMessage struct {
	Type         MessageType `json:"type"`
	PairingToken string      `json:"pairing_token"`
	PIN          string      `json:"pin"`
	DeviceName   string      `json:"device_name,omitempty"`
}

// PairResponseMessage is returned by the host upon PIN verification.
type PairResponseMessage struct {
	Type      MessageType            `json:"type"`
	Success   bool                   `json:"success"`
	SessionID string                 `json:"session_id,omitempty"`
	Error     string                 `json:"error,omitempty"`
	State     *ppt.PresentationState `json:"state,omitempty"`
}

// ResumeRequestMessage allows reconnecting without re-entering PIN.
type ResumeRequestMessage struct {
	Type      MessageType `json:"type"`
	SessionID string      `json:"session_id"`
}

// ActionMessage carries slide navigation commands.
type ActionMessage struct {
	Type        MessageType `json:"type"`
	Action      ppt.Action  `json:"action"`
	TargetSlide int         `json:"target_slide,omitempty"`
}

// StateUpdateMessage sends presentation telemetry to the remote.
type StateUpdateMessage struct {
	Type  MessageType           `json:"type"`
	State ppt.PresentationState `json:"state"`
}

// PingMessage / PongMessage for latency monitoring.
type PingMessage struct {
	Type      MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
}

type PongMessage struct {
	Type      MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
}

type AlertMessage struct {
	Type    MessageType `json:"type"`
	Message string      `json:"message"`
}
