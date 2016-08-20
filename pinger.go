package main

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/mapper"
)

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
		routeKeyParts := strings.SplitN(routeKey, ",", 2)
		srcKeyParts := strings.SplitN(routeKeyParts[0], ":", 2)
		srcPort, _ := strconv.Atoi(srcKeyParts[1])
		logrus.Debugf("Ping src=%s dst=%s", routeKeyParts[0], routeKeyParts[1])

		p := ping{
			SrcName:    p.Self.Name,
			SrcPort:    srcPort,
			DstName:    peer.Name,
			DstPort:    peer.Port,
			Path:       route.Hops(),
			PingTimeNS: time.Now().UnixNano(),
		}
		// TODO: major cleanup to encapsulate all this message sending
		// Encode as a user message
		encodedBuf, err := encode(pingMsg, p)
		if err != nil {
			logrus.Infof("Unable to encode pingMsg: %v", err)
			continue
		}
		msg := encodedBuf.Bytes()
		buf := make([]byte, 1, len(msg)+1)
		buf[0] = byte(8) // TODO: add sendFrom API to memberlist
		buf = append(buf, msg...)

		LocalAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+srcKeyParts[1])
		if err != nil {
			logrus.Errorf("unable to resolve source addr %v", err)
		}
		RemoteEP := net.UDPAddr{
			IP:   net.ParseIP(peer.Name), // TODO: have ip field
			Port: peer.Port,
		}
		conn, err := net.DialUDP("udp", LocalAddr, &RemoteEP)
		if err != nil {
			// handle error
			logrus.Errorf("unable to connect to peer: %v", err)
			continue
		}
		// TODO: configurable time
		conn.SetDeadline(time.Now().Add(time.Second))

		conn.Write(buf)

		// get a response from the ping
		retBuf := make([]byte, 2048)
		readRet, err := conn.Read(retBuf)

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
					logrus.Warning("Unable to decode message: %v", err)
				} else {
					passed = true
				}

			default:
				logrus.Infof("Got unknown response type from ack: %v", msgType)
			}
		}
		route.HandleACK(passed, time.Now().UnixNano()-p.PingTimeNS)
		conn.Close()
		time.Sleep(time.Second)
	}
}
