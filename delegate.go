package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"
	"github.com/jacksontj/dnms/graph"
)

type DNMSDelegate struct {
	Graph    *graph.NetworkGraph
	RouteMap *RouteMap
}

func NewDNMSDelegate(g *graph.NetworkGraph, r *RouteMap) *DNMSDelegate {
	return &DNMSDelegate{
		Graph:    g,
		RouteMap: r,
	}
}

// NodeMeta is used to retrieve meta-data about the current node
// when broadcasting an alive message. It's length is limited to
// the given byte size. This metadata is available in the Node structure.
func (d *DNMSDelegate) NodeMeta(limit int) []byte {
	return nil
}

// NotifyMsg is called when a user-data message is received.
// Care should be taken that this method does not block, since doing
// so would block the entire UDP packet receive loop. Additionally, the byte
// slice may be modified after the call returns, so it should be copied if needed.
func (d *DNMSDelegate) NotifyMsg(buf []byte) {
	msgType := messageType(buf[0])
	buf = buf[1:]

	switch msgType {

	case pingMsg:
		p := ping{}
		err := decode(buf, &p)
		if err != nil {
			logrus.Warning("Unable to decode message: %v", err)
			return
		}
		logrus.Infof("Got a ping on %v", p)

		// TODO: send the ack back the same way it came (if possible??)
		// from some limited testing this seems VERY unreliable-- we should either
		// just send it back however or from a specific port
		routeKey, ok := d.RouteMap.FindRoute(p.Path)
		logrus.Infof("Reverse route? routeKey=%s ok=%v", routeKey, ok)

		msg := []byte("ack")
		buf := make([]byte, 1, len(msg)+1)
		buf[0] = byte(8) // TODO: add sendFrom API to memberlist
		buf = append(buf, msg...)

		// TODO: must have same source port :( This is either going to require work
		// to memberlist, or a separate set of ports for our messages vs gossip
		// or that ACKs go through an unknown path (probably not preferred -- consistent is better)
		/*
			// write return packet
			RemoteEP := net.UDPAddr{IP: net.ParseIP(p.SrcName), Port: int(p.SrcPort)}
			conn, err := net.DialUDP("udp", &net.UDPAddr{}, &RemoteEP)
			if err != nil {
				// handle error
				logrus.Errorf("unable to connect to peer: %v", err)
				return
			}
			// TODO: configurable time
			//conn.SetDeadline(time.Now().Add(time.Second))

			conn.Write(buf)
		*/
	default:
		logrus.Infof("Unknown messageType=%d", msgType)

	}

}

// GetBroadcasts is called when user data messages can be broadcast.
// It can return a list of buffers to send. Each buffer should assume an
// overhead as provided with a limit on the total byte size allowed.
// The total byte size of the resulting data to send must not exceed
// the limit. Care should be taken that this method does not block,
// since doing so would block the entire UDP packet receive loop.
func (d *DNMSDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

// LocalState is used for a TCP Push/Pull. This is sent to
// the remote side in addition to the membership information. Any
// data can be sent here. See MergeRemoteState as well. The `join`
// boolean indicates this is for a join instead of a push/pull.
func (d *DNMSDelegate) LocalState(join bool) []byte {
	return nil
}

// MergeRemoteState is invoked after a TCP Push/Pull. This is the
// state received from the remote side and is the result of the
// remote side's LocalState call. The 'join'
// boolean indicates this is for a join instead of a push/pull.
func (d *DNMSDelegate) MergeRemoteState(buf []byte, join bool) {

}

// Event delegate methods
// NotifyJoin is invoked when a node is detected to have joined.
// The Node argument must not be modified.
func (d *DNMSDelegate) NotifyJoin(n *memberlist.Node) {
	logrus.Infof("Node joined %s", n.Addr.String())
}

// NotifyLeave is invoked when a node is detected to have left.
// The Node argument must not be modified.
func (d *DNMSDelegate) NotifyLeave(n *memberlist.Node) {
	logrus.Infof("Node left %s", n.Addr.String())
}

// NotifyUpdate is invoked when a node is detected to have
// updated, usually involving the meta data. The Node argument
// must not be modified.
func (d *DNMSDelegate) NotifyUpdate(*memberlist.Node) {
}
