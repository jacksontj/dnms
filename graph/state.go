package graph

type graphState uint8

const (
	Up graphState = iota
	Suspect
	Down
)
