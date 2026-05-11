package main

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jacksontj/dnms/mapper"
	"github.com/sirupsen/logrus"
)

func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

// TODO: move to another package??

type Pinger struct {
	M *mapper.Mapper
	T *PingTransport

	Self mapper.Peer

	// Tunables. Zero values fall back to defaults applied in Start().
	// PeerInterval is the pause between successive peers in a single sweep.
	PeerInterval time.Duration
	// RouteInterval is the pause between successive routes when pinging one
	// peer (each peer has up to SrcPortEnd-SrcPortStart routes to cycle through).
	RouteInterval time.Duration
	// Timeout bounds how long we wait for an ack before declaring loss.
	Timeout time.Duration
}

// Start begins pinging peers. The pinger exits when ctx is canceled.
func (p *Pinger) Start(ctx context.Context) {
	if p.PeerInterval <= 0 {
		p.PeerInterval = 100 * time.Millisecond
	}
	if p.RouteInterval <= 0 {
		p.RouteInterval = time.Second
	}
	if p.Timeout <= 0 {
		p.Timeout = time.Second
	}
	go p.PingPeers(ctx)
}

// ping all the things
func (p *Pinger) PingPeers(ctx context.Context) {
	for ctx.Err() == nil {
		peerChan := p.M.IterPeers()
		for peer := range peerChan {
			if ctx.Err() != nil {
				return
			}
			p.PingPeer(ctx, peer)
			select {
			case <-ctx.Done():
				return
			case <-time.After(p.PeerInterval):
			}
		}
	}
}

func (p *Pinger) PingPeer(ctx context.Context, peer *mapper.Peer) {
	c := make(chan string)
	p.M.RouteMap.IterRoutes(peer.String(), c)
	for routeKey := range c {
		if ctx.Err() != nil {
			return
		}
		route := p.M.RouteMap.GetRoute(routeKey)
		// TODO: better
		if route == nil {
			continue
		}
		srcEnd, dstEnd, ok := strings.Cut(routeKey, ",")
		if !ok {
			logrus.Warningf("malformed route key (no comma): %q", routeKey)
			continue
		}
		_, srcPortStr, err := net.SplitHostPort(srcEnd)
		if err != nil {
			logrus.Warningf("malformed route key src %q: %v", srcEnd, err)
			continue
		}
		srcPort, err := strconv.Atoi(srcPortStr)
		if err != nil {
			logrus.Warningf("malformed src port in route key %q: %v", srcEnd, err)
			continue
		}
		logrus.Debugf("Ping src=%s dst=%s", srcEnd, dstEnd)

		pkt := ping{
			SrcName:    p.Self.Name,
			SrcPort:    srcPort,
			DstName:    peer.Name,
			DstPort:    peer.Port,
			Path:       route.Hops(),
			PingTimeNS: time.Now().UnixNano(),
		}

		_, rtt, err := p.T.SendPing(pkt, peer.Name, srcPort, p.Timeout)
		passed := err == nil
		if err != nil {
			// EADDRINUSE just means another goroutine is using this source port
			// right now; the loop will retry on the next pass. Anything else
			// (including timeout) is treated as a missed ack — the route's
			// HandleACK will record that as a sample.
			if isAddrInUse(err) {
				logrus.Debugf("source port %d in use, skipping: %v", srcPort, err)
			} else {
				logrus.Debugf("ping %s -> %s failed: %v", srcEnd, peer.String(), err)
			}
		}

		var latency int64
		if passed {
			latency = int64(rtt)
		} else {
			// Use the elapsed wall time as a coarse "latency" sample so the
			// ring keeps pace; downstream code differentiates pass vs. fail.
			latency = time.Now().UnixNano() - pkt.PingTimeNS
		}
		route.HandleACK(passed, latency)

		select {
		case <-ctx.Done():
			return
		case <-time.After(p.RouteInterval):
		}
	}
}
