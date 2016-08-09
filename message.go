package main

// messageType is an integer ID of a type of message that can be received
// on network channels from other members.
type messageType uint8

const (
	pingMsg messageType = iota
	ackMsg
)

type ping struct {
	// Node is sent so the target can verify they are
	// the intended recipient. This is to protect again an agent
	// restart with a new name.
	SrcName string
	SrcPort int

	DstName string
	Path    []string
}
