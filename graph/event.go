package graph

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
)

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

func (e Event) Id() string {
	// we don't care about event replay for now
	return ""
}

func (e Event) Event() string {
	switch e.Item.(type) {
	case *NetworkNode:
		switch e.E {
		case addEvent:
			return "addNodeEvent"
		case updateEvent:
			return "updateNodeEvent"
		case removeEvent:
			return "removeNodeEvent"
		}
	case *NetworkLink:
		switch e.E {
		case addEvent:
			return "addLinkEvent"
		case updateEvent:
			return "updateLinkEvent"
		case removeEvent:
			return "removeLinkEvent"
		}
	case *NetworkRoute:
		switch e.E {
		case addEvent:
			return "addRouteEvent"
		case updateEvent:
			return "updateRouteEvent"
		case removeEvent:
			return "removeRouteEvent"
		}
	}

	logrus.Warning("Unknown event type!")
	return "unknown"
}

func (e Event) Data() string {
	ret, err := json.Marshal(e.Item)
	if err != nil {
		logrus.Warningf("Unable to marshal event: %v", err)
		return ""
	} else {
		return string(ret)
	}
}
