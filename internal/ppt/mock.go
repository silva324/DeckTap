package ppt

import (
	"context"
	"sync"
)

type mockSlide struct {
	Title string
	Notes string
}

// MockController simulates PowerPoint with a realistic presentation deck.
type MockController struct {
	mu           sync.Mutex
	available    bool
	running      bool
	currentSlide int
	fileName     string
	blackScreen  bool
	whiteScreen  bool
	slides       []mockSlide
}

func NewMockController() *MockController {
	return &MockController{
		available:    true,
		running:      true,
		currentSlide: 1,
		fileName:     "Zero_Trust_E2EE_Architecture.pptx",
		slides: []mockSlide{
			{
				Title: "Zero-Trust PowerPoint Remote: Architecture",
				Notes: "Welcome everyone! Today we introduce a zero-trust, end-to-end encrypted presentation remote.\n\nKey talking points:\n• No third-party servers see unencrypted presentation data or speaker notes.\n• Single binary host agent with zero dependencies on macOS and Windows.\n• Seamless QR scan pairing combined with a 4-digit visual PIN.",
			},
			{
				Title: "E2EE Wire Protocol: AES-256-GCM",
				Notes: "Dive into the cryptography:\n• Each session generates a fresh 256-bit AES MasterKey (crypto/rand).\n• Browser Web Crypto API (SubtleCrypto) handles client-side encrypt/decrypt.\n• Every WebSocket payload includes a unique 12-byte IV and 16-byte authentication tag.\n• Zero plaintext is ever leaked to proxies or tunnel relays.",
			},
			{
				Title: "Out-of-Band Secrets & URL Fragments",
				Notes: "Explain the URI fragment mechanism:\n• The MasterKey and PairingToken are transmitted in the URL #fragment.\n• RFC 3986 guarantees HTTP clients and proxies NEVER send URL fragments over the wire.\n• Even Cloudflare edge servers cannot see the keys in request logs.",
			},
			{
				Title: "Anti-Shoulder-Surfing Visual PIN UX",
				Notes: "Address the shoulder-surfing threat:\n• What if someone in the auditorium photos the QR code on the presenter's screen?\n• A mandatory 4-digit PIN displayed exclusively on the presenter's laptop screen prevents unauthorized pairing.\n• Brute-force protection: 5 failed attempts trigger rate-limiting and temporary lockout.",
			},
			{
				Title: "Autonomous Cloudflare Tunnel Auto-Fetcher",
				Notes: "Zero-config connectivity:\n• If cloudflared is not installed on the system, the host agent autonomously downloads the official release for macOS/Windows.\n• Automatically runs a temporary Quick Tunnel with valid public HTTPS/WSS.\n• Automatically falls back to local LAN IP if offline or blocked.",
			},
			{
				Title: "Native OS Control: AppleScript & COM",
				Notes: "Control Microsoft PowerPoint natively without any plugins or add-ins:\n• macOS: Pure AppleScript (osascript) querying slide indexes and notes pages.\n• Windows: PowerShell COM automation targeting PowerPoint.Application.\n• Real-time two-way synchronization of current slide, total count, and speaker notes.",
			},
			{
				Title: "Mobile Presentation Experience",
				Notes: "Highlight the speaker's mobile UI:\n• Huge tactile next/previous touch targets with swipe gestures.\n• High-contrast dark theme so phone glow doesn't distract the audience.\n• Next-slide peek gives confidence on transitions.\n• Built-in stopwatch and timer keeps presentations on track.",
			},
			{
				Title: "Summary & Open Questions",
				Notes: "Wrap up:\n• Review security properties.\n• Solicit questions from attendees.\n• Provide contact info and GitHub repository link.",
			},
		},
	}
}

func (m *MockController) Name() string {
	return "Mock Presentation Engine (In-Memory Simulator)"
}

func (m *MockController) GetState(ctx context.Context) (PresentationState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.buildStateLocked(), nil
}

func (m *MockController) ExecuteAction(ctx context.Context, action Action, targetSlide int) (PresentationState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch action {
	case ActionNextSlide:
		if m.currentSlide < len(m.slides) {
			m.currentSlide++
		}
		m.blackScreen = false
		m.whiteScreen = false
	case ActionPrevSlide:
		if m.currentSlide > 1 {
			m.currentSlide--
		}
		m.blackScreen = false
		m.whiteScreen = false
	case ActionGotoSlide:
		if targetSlide >= 1 && targetSlide <= len(m.slides) {
			m.currentSlide = targetSlide
		}
		m.blackScreen = false
		m.whiteScreen = false
	case ActionStartShow:
		m.running = true
		m.blackScreen = false
		m.whiteScreen = false
	case ActionEndShow:
		m.running = false
	case ActionBlackScreen:
		m.blackScreen = !m.blackScreen
		m.whiteScreen = false
	case ActionWhiteScreen:
		m.whiteScreen = !m.whiteScreen
		m.blackScreen = false
	}

	return m.buildStateLocked(), nil
}

func (m *MockController) buildStateLocked() PresentationState {
	total := len(m.slides)
	if total == 0 {
		return PresentationState{
			Available:    m.available,
			Running:      m.running,
			CurrentSlide: 0,
			TotalSlides:  0,
			Title:        "No Presentation",
			Notes:        "",
			FileName:     m.fileName,
			BlackScreen:  m.blackScreen,
			WhiteScreen:  m.whiteScreen,
		}
	}

	curIdx := m.currentSlide - 1
	if curIdx < 0 {
		curIdx = 0
	}
	if curIdx >= total {
		curIdx = total - 1
	}

	curSlide := m.slides[curIdx]

	nextTitle := ""
	nextNotes := ""
	if curIdx+1 < total {
		nextSlide := m.slides[curIdx+1]
		nextTitle = nextSlide.Title
		nextNotes = nextSlide.Notes
	}

	titles := make([]string, len(m.slides))
	for i, s := range m.slides {
		titles[i] = s.Title
	}

	return PresentationState{
		Available:    m.available,
		Running:      m.running,
		CurrentSlide: m.currentSlide,
		TotalSlides:  total,
		Title:        curSlide.Title,
		Notes:        curSlide.Notes,
		NextTitle:    nextTitle,
		NextNotes:    nextNotes,
		FileName:     m.fileName,
		BlackScreen:  m.blackScreen,
		WhiteScreen:  m.whiteScreen,
		SlideTitles:  titles,
	}
}
