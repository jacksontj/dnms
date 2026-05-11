package main

import (
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

	Self mapper.Peer
}

func (p *Pinger) Start() {
	go p.PingPeers()
}

// TODO implement
func (p *Pinger) Stop() {

}

// ping all the things
func (p *Pinger) PingPeers() {
	for {
		peerChan := p.M.IterPeers()
		for peer := range peerChan {
			p.PingPeer(peer)
			// TODO configurable rate
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func (p *Pinger) PingPeer(peer *mapper.Peer) {
	c := make(chan string)
	p.M.RouteMap.IterRoutes(peer.String(), c)
	for routeKey := range c {
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
		// TODO: major cleanup to encapsulate all this message sending
		// Encode as a user message
		encodedBuf, err := encode(pingMsg, pkt)
		if err != nil {
			logrus.Infof("Unable to encode pingMsg: %v", err)
			continue
		}
		msg := encodedBuf.Bytes()
		buf := make([]byte, 1, len(msg)+1)
		buf[0] = byte(8) // TODO: add sendFrom API to memberlist
		buf = append(buf, msg...)

		localAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort("0.0.0.0", srcPortStr))
		if err != nil {
			logrus.Errorf("unable to resolve source addr: %v", err)
			continue
		}
		remoteEP := net.UDPAddr{
			IP:   net.ParseIP(peer.Name), // TODO: have ip field
			Port: peer.Port,
		}
		conn, err := net.DialUDP("udp", localAddr, &remoteEP)
		if err != nil {
			// EADDRINUSE happens when another goroutine is already bound to
			// this source port (or recently was); log at debug since the next
			// iteration retries with a different source port. Anything else
			// is a real error.
			if isAddrInUse(err) {
				logrus.Debugf("source port %d in use, will retry: %v", srcPort, err)
			} else {
				logrus.Errorf("unable to connect to peer: %v", err)
			}
			continue
		}
		// TODO: configurable time
		conn.SetDeadline(time.Now().Add(time.Second))

		// TODO: check for error
		conn.Write(buf)

		// get a response from the ping
		retBuf := make([]byte, 2048)
		// TODO: something with err
		readRet, _ := conn.Read(retBuf)

		// Whether we got an ACK
		passed := false

		// if there was a response
		if readRet > 0 {
			// Note: throwing away the first byte-- as its the memberlist header
			msgType := messageType(retBuf[1])
			retBuf = retBuf[2:]

			switch msgType {

			case ackMsg:
				a := ack{}
				err := decode(retBuf, &a)
				if err != nil {
					logrus.Warningf("Unable to decode message: %v", err)
				} else {
					passed = true
				}

			default:
				logrus.Infof("Got unknown response type from ack: %v", msgType)
			}
		}
		route.HandleACK(passed, time.Now().UnixNano()-pkt.PingTimeNS)
		conn.Close()
		// TODO: configurable ping sleep
		time.Sleep(time.Second)
	}
}
