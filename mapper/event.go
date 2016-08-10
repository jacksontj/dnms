package mapper

// TODO: implement
// Mapper events (when it changes the graph)
type eventType uint8

const (
	addEvent eventType = iota
	updateEvent
	removeEvent
)

type Event struct {
	E eventType
	// Pointer to the thing that changed
	Item interface{}
}
