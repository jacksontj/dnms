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
		peerChan := make(chan *mapper.Peer)
		p.M.IterPeers(peerChan)
		for peer := range peerChan {
			p.PingPeer(peer)
			// TODO configurable rate
			time.Sleep(time.Second * 1)
		}
	}
}

func (p *Pinger) PingPeer(peer *mapper.Peer) {
	c := make(chan string)
	p.M.RouteMap.IterRoutes(peer.Name, c)
	for routeKey := range c {
		route := p.M.RouteMap.GetRoute(routeKey)
		// TODO: better
		if route == nil {
			continue
		}
		routeKeyParts := strings.SplitN(routeKey, ":", 2)
		srcPort, _ := strconv.Atoi(routeKeyParts[0])
		logrus.Infof("Ping srcPort=%d dst=%s", srcPort, routeKeyParts[1])

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

		LocalAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+routeKeyParts[0])
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

		// TODO: figure out how to get the message back...
		// seems that I might have to add some methods to get at `WriteToUDP`
		// in memberlist
		conn.Write(buf)

		// TODO: get a response from the ping
		retBuf := make([]byte, 2048)
		readRet, err := conn.Read(retBuf)
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
					continue
				} else {
					//logrus.Infof("took %v ns): %v", time.Now().UnixNano()-a.PingTimeNS, a)
					// TODO: use the ACK for something
				}

			default:
				logrus.Infof("Got unknown response type from ack: %v", msgType)
			}
		} else {
			// TODO: use the absense of ACK for something
			logrus.Infof("ACK timeout")
		}
		conn.Close()
		time.Sleep(time.Second)
	}
}
