package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// PingTransport owns a dedicated UDP listener for ping/ack exchange. It
// replaces the prior arrangement where dnms piggy-backed on memberlist's UDP
// receive loop (and required a forked memberlist with SendToUDPPort). All
// nodes in a cluster must agree on the same listen port.
type PingTransport struct {
	listenPort int

	// Set after Listen returns; nil before.
	conn *net.UDPConn
}

func NewPingTransport(listenPort int) *PingTransport {
	return &PingTransport{listenPort: listenPort}
}

// Listen binds a UDP socket on the configured port and starts the receive
// loop in a goroutine. The receive loop exits when ctx is canceled.
func (t *PingTransport) Listen(ctx context.Context) error {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: t.listenPort}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("ping transport listen %s: %w", addr, err)
	}
	t.conn = conn

	go func() {
		<-ctx.Done()
		// Closing the conn unblocks ReadFromUDP so the receive goroutine exits.
		conn.Close()
	}()

	go t.serve(ctx)
	return nil
}

func (t *PingTransport) serve(ctx context.Context) {
	buf := make([]byte, 2048)
	for {
		n, src, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logrus.Errorf("ping transport read: %v", err)
			continue
		}
		if n < 1 {
			continue
		}
		// First byte is the message type, rest is msgpack-encoded payload.
		msgType := messageType(buf[0])
		if msgType != pingMsg {
			// Acks should be received via the per-send socket, not here.
			logrus.Debugf("ping transport got unexpected msgType=%d from %s", msgType, src)
			continue
		}
		var p ping
		if err := decode(buf[1:n], &p); err != nil {
			logrus.Warningf("ping transport decode: %v", err)
			continue
		}
		t.handlePing(p, src)
	}
}

// handlePing answers a received ping with an ack, using the source address
// the ping arrived from (so we don't trust the SrcName field, and IPv4/IPv6
// just works).
func (t *PingTransport) handlePing(p ping, src *net.UDPAddr) {
	a := ack{PingTimeNS: p.PingTimeNS}
	encoded, err := encode(ackMsg, a)
	if err != nil {
		logrus.Errorf("ping transport encode ack: %v", err)
		return
	}
	if _, err := t.conn.WriteToUDP(encoded.Bytes(), src); err != nil {
		logrus.Errorf("ping transport write ack to %s: %v", src, err)
	}
}

// SendPing dispatches a single ping to dstHost on the configured listen port,
// from a socket bound to "0.0.0.0:srcPort" (so we elicit ECMP variation).
// Returns the ack and the measured RTT. err is non-nil on send/read/decode
// failure or timeout.
func (t *PingTransport) SendPing(p ping, dstHost string, srcPort int, timeout time.Duration) (ack, time.Duration, error) {
	encoded, err := encode(pingMsg, p)
	if err != nil {
		return ack{}, 0, fmt.Errorf("encode: %w", err)
	}

	localAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort("0.0.0.0", strconv.Itoa(srcPort)))
	if err != nil {
		return ack{}, 0, fmt.Errorf("resolve local: %w", err)
	}
	dstIP := net.ParseIP(dstHost)
	if dstIP == nil {
		return ack{}, 0, fmt.Errorf("invalid dst host %q", dstHost)
	}
	remote := &net.UDPAddr{IP: dstIP, Port: t.listenPort}

	conn, err := net.DialUDP("udp", localAddr, remote)
	if err != nil {
		return ack{}, 0, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))
	start := time.Now()
	if _, err := conn.Write(encoded.Bytes()); err != nil {
		return ack{}, 0, fmt.Errorf("write: %w", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return ack{}, 0, err
	}
	rtt := time.Since(start)
	if n < 1 {
		return ack{}, rtt, errors.New("empty response")
	}
	if messageType(buf[0]) != ackMsg {
		return ack{}, rtt, fmt.Errorf("unexpected msg type %d in response", buf[0])
	}
	var a ack
	if err := decode(buf[1:n], &a); err != nil {
		return ack{}, rtt, fmt.Errorf("decode: %w", err)
	}
	return a, rtt, nil
}
