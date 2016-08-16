package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/memberlist"
)

type AggregatorDelegate struct {
	P *PeerGraphMap

	peerSubs map[*memberlist.Node]chan bool

	Mlist *memberlist.Memberlist
}

func NewAggregatorDelegate(p *PeerGraphMap) *AggregatorDelegate {
	return &AggregatorDelegate{
		P:        p,
		peerSubs: make(map[*memberlist.Node]chan bool),
	}
}

// NodeMeta is used to retrieve meta-data about the current node
// when broadcasting an alive message. It's length is limited to
// the given byte size. This metadata is available in the Node structure.
func (d *AggregatorDelegate) NodeMeta(limit int) []byte {
	return nil
}

// NotifyMsg is called when a user-data message is received.
// Care should be taken that this method does not block, since doing
// so would block the entire UDP packet receive loop. Additionally, the byte
// slice may be modified after the call returns, so it should be copied if needed.
func (d *AggregatorDelegate) NotifyMsg(buf []byte) {

}

// GetBroadcasts is called when user data messages can be broadcast.
// It can return a list of buffers to send. Each buffer should assume an
// overhead as provided with a limit on the total byte size allowed.
// The total byte size of the resulting data to send must not exceed
// the limit. Care should be taken that this method does not block,
// since doing so would block the entire UDP packet receive loop.
func (d *AggregatorDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

// LocalState is used for a TCP Push/Pull. This is sent to
// the remote side in addition to the membership information. Any
// data can be sent here. See MergeRemoteState as well. The `join`
// boolean indicates this is for a join instead of a push/pull.
func (d *AggregatorDelegate) LocalState(join bool) []byte {
	return nil
}

// MergeRemoteState is invoked after a TCP Push/Pull. This is the
// state received from the remote side and is the result of the
// remote side's LocalState call. The 'join'
// boolean indicates this is for a join instead of a push/pull.
func (d *AggregatorDelegate) MergeRemoteState(buf []byte, join bool) {

}

// Event delegate methods
// NotifyJoin is invoked when a node is detected to have joined.
// The Node argument must not be modified.
func (d *AggregatorDelegate) NotifyJoin(n *memberlist.Node) {
	// Workaround startup chicken and egg problem
	if d.Mlist == nil {
		return
	}
	logrus.Infof("Node joined %s", n.Addr.String())
	if _, ok := d.peerSubs[n]; ok {
		logrus.Infof("Node joined that we are already subscribed to!")
		return
	}
	c := subscribe(d.P, "http://"+n.Addr.String()+":12345/v1/events/graph")
	d.peerSubs[n] = c
}

// NotifyLeave is invoked when a node is detected to have left.
// The Node argument must not be modified.
func (d *AggregatorDelegate) NotifyLeave(n *memberlist.Node) {
	logrus.Infof("Node left %s", n.Addr.String())
	if exitChan, ok := d.peerSubs[n]; !ok {
		logrus.Infof("Node leaving that we aren't subscribed to!")
	} else {
		exitChan <- true
		delete(d.peerSubs, n)
	}
}

// NotifyUpdate is invoked when a node is detected to have
// updated, usually involving the meta data. The Node argument
// must not be modified.
func (d *AggregatorDelegate) NotifyUpdate(*memberlist.Node) {
}
