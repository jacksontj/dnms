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
	Path    []string
}
