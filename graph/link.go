// TODO: better name? network topology?
package graph

type NetworkLink struct {
	SrcName string `json:"src"`
	srcNode *NetworkNode
	DstName string `json:"dst"`
	dstNode *NetworkNode

	refCount int
}

// TODO: use? right now it embeds the network node in the response-- which is duplicated data
/*
// Fancy marshal method
func (l *NetworkLink) MarshalJSON() ([]byte, error) {

	type Alias NetworkLink
	return json.Marshal(&struct {
		Src string
		Dst string
	}{
		Src: l.Src.Name,
		Dst: l.Dst.Name,
	})
}
*/
