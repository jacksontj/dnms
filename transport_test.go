package main

import (
	"context"
	"net"
	"testing"
	"time"
)

// pickFreePort grabs an OS-assigned UDP port for the test, then closes the
// socket so the transport can rebind. Race-window is small enough for tests.
func pickFreePort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("pickFreePort: %v", err)
	}
	port := c.LocalAddr().(*net.UDPAddr).Port
	c.Close()
	return port
}

// TestTransportRoundTrip starts a single transport, sends itself a ping, and
// checks the ack comes back with the same PingTimeNS — exercising the full
// listen/decode/encode/respond loop.
func TestTransportRoundTrip(t *testing.T) {
	port := pickFreePort(t)
	srcPort := pickFreePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tr := NewPingTransport(port)
	if err := tr.Listen(ctx); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	// Give the listener a moment to enter its read loop.
	time.Sleep(50 * time.Millisecond)

	const pingTime int64 = 1234567890
	p := ping{
		SrcName:    "127.0.0.1",
		SrcPort:    srcPort,
		DstName:    "127.0.0.1",
		DstPort:    port,
		Path:       []string{"127.0.0.1"},
		PingTimeNS: pingTime,
	}

	a, rtt, err := tr.SendPing(p, "127.0.0.1", srcPort, time.Second)
	if err != nil {
		t.Fatalf("SendPing: %v", err)
	}
	if a.PingTimeNS != pingTime {
		t.Errorf("ack PingTimeNS=%d, want %d", a.PingTimeNS, pingTime)
	}
	if rtt <= 0 || rtt > time.Second {
		t.Errorf("rtt out of range: %v", rtt)
	}
}

// TestTransportSendPingTimeout confirms SendPing returns an error (rather than
// hanging) when no listener answers — the receive deadline fires.
func TestTransportSendPingTimeout(t *testing.T) {
	dstPort := pickFreePort(t)
	srcPort := pickFreePort(t)

	tr := NewPingTransport(dstPort) // never Listen()'d, so nothing answers

	p := ping{
		SrcName:    "127.0.0.1",
		SrcPort:    srcPort,
		DstName:    "127.0.0.1",
		DstPort:    dstPort,
		PingTimeNS: time.Now().UnixNano(),
	}

	start := time.Now()
	_, _, err := tr.SendPing(p, "127.0.0.1", srcPort, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}
