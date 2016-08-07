// TODO: better name? network topology?
package graph

// TODO: overload equality operator to not care about directions
type NetworkLinkKey struct {
	Src string
	Dst string
}

type NetworkLink struct {
	Src *NetworkNode
	Dst *NetworkNode

	RefCount int
}
