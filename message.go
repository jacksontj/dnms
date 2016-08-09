package main

// messageType is an integer ID of a type of message that can be received
// on network channels from other members.
type messageType uint8

const (
	pingMsg messageType = iota
	ackMsg
)

type ping struct {
	SrcName string
	SrcPort int

	DstName string
	DstPort int
	// TODO: don't send? seems that the peers don't usually share paths
	Path []string

	// Note: we can't use the times anywhere but where they where measured-- due
	// to clock drift
	PingTimeNS int64
}

type ack struct {
	PingTimeNS int64

	// TODO: don't send? seems that the peers don't usually share paths
	Path []string
}
