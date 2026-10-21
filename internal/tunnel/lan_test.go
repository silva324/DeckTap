package tunnel

import (
	"net"
	"testing"
)

func TestGetPreferredOutboundIP(t *testing.T) {
	ipStr := GetPreferredOutboundIP()
	if ipStr == "" {
		t.Fatal("expected non-empty IP string")
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		t.Fatalf("expected valid IP address, got: %q", ipStr)
	}

	// Should be an IPv4 address
	if ip.To4() == nil {
		t.Fatalf("expected IPv4 address, got: %q", ipStr)
	}
}
