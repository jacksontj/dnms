package main

import (
	"sync/atomic"

	"github.com/jacksontj/dnms/aggregator"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/memberlist"
	"github.com/sirupsen/logrus"
)

type DNMSDelegate struct {
	// for us to map things
	Mapper *mapper.Mapper

	// for aggregation
	AggMap *aggregator.AggGraphMap

	// memberlist.Create can fire NotifyMsg/NotifyJoin during construction —
	// before the caller has a chance to wire up the *Memberlist value — so we
	// store it atomically and let those callbacks observe nil safely until
	// SetMlist is called.
	mlist atomic.Pointer[memberlist.Memberlist]
}

func NewDNMSDelegate(m *mapper.Mapper, a *aggregator.AggGraphMap) *DNMSDelegate {
	return &DNMSDelegate{
		Mapper: m,
		AggMap: a,
	}
}

func (d *DNMSDelegate) SetMlist(m *memberlist.Memberlist) {
	d.mlist.Store(m)
}

func (d *DNMSDelegate) Mlist() *memberlist.Memberlist {
	return d.mlist.Load()
}

// NodeMeta is used to retrieve meta-data about the current node
// when broadcasting an alive message. It's length is limited to
// the given byte size. This metadata is available in the Node structure.
func (d *DNMSDelegate) NodeMeta(limit int) []byte {
	return nil
}

// NotifyMsg is called when a user-data message is received via memberlist.
// Ping/ack used to be carried here; now they have their own UDP transport,
// so this is a no-op left in place for any future memberlist user messages.
func (d *DNMSDelegate) NotifyMsg(buf []byte) {
	if len(buf) == 0 {
		return
	}
	logrus.Debugf("ignoring memberlist user msg type=%d (no handler)", buf[0])
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
	// Memberlist may fire NotifyJoin before SetMlist has been called.
	if d.Mlist() == nil {
		return
	}
	logrus.Infof("Node joined %s", n.Addr.String())
	// TOOD: check it isn't us? shouldn't be as that should be covered by our
	// workaround up top
	go d.Mapper.AddPeer(mapper.Peer{
		Name: n.Addr.String(),
		Port: int(n.Port),
	})

	// if we are an aggregator
	if d.AggMap != nil {
		d.AggMap.AddPeer(n.Addr.String())
	}
}

// NotifyLeave is invoked when a node is detected to have left.
// The Node argument must not be modified.
func (d *DNMSDelegate) NotifyLeave(n *memberlist.Node) {
	logrus.Infof("Node left %s", n.Addr.String())
	go d.Mapper.RemovePeer(mapper.Peer{
		Name: n.Addr.String(),
		Port: int(n.Port),
	})
	// if we are an aggregator
	if d.AggMap != nil {
		d.AggMap.RemovePeer(n.Addr.String())
	}
}

// NotifyUpdate is invoked when a node is detected to have
// updated, usually involving the meta data. The Node argument
// must not be modified.
func (d *DNMSDelegate) NotifyUpdate(*memberlist.Node) {
}
