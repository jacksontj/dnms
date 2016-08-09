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
	logrus.Infof("Got msg: %v", buf)

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
