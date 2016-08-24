package graph

import (
	"fmt"
	"strings"
)

const UNKNOWN_PATH = "*"

// Better name
type pathMergePuzzlePiece struct {
	// a value we have
	Val string
	// a requirement we have from some other key
	Req map[string]interface{}
}

func newPathMergePuzzlePiece(v string) *pathMergePuzzlePiece {
	return &pathMergePuzzlePiece{
		Val: v,
		Req: make(map[string]interface{}),
	}
}

func (p *pathMergePuzzlePiece) Valid() bool {
	for v, _ := range p.Req {
		// requisites of "*" mean nothing
		if v == UNKNOWN_PATH {
			continue
		}
		if v != p.Val {
			return false
		}
	}
	return true
}

// Create a "puzzle" for the N constrained list of paths
func CreatePuzzle(paths ...[]string) map[int]*pathMergePuzzlePiece {
	puzzle := make(map[int]*pathMergePuzzlePiece)
	for _, path := range paths {
		for i, v := range path {
			if strings.Contains(v, UNKNOWN_PATH) {
				// set a value for the piece if one doesn't exist
				parentPiece, ok := puzzle[i]
				if !ok {
					parentPiece = newPathMergePuzzlePiece(UNKNOWN_PATH)
					puzzle[i] = parentPiece
				} else {
					if parentPiece.Val == "" {
						parentPiece.Val = UNKNOWN_PATH
					}
				}

				hopParts := strings.SplitN(v, "|", 3)
				var prefixParts, suffixParts []string
				if strings.Contains(hopParts[0], ",") {
					prefixParts = strings.Split(hopParts[0], ",")
				}
				if strings.Contains(hopParts[2], ",") {
					suffixParts = strings.Split(hopParts[2], ",")
				}

				// add prefix req
				for x, prefixPart := range prefixParts {
					key := i - (len(prefixParts) - x)
					piece, ok := puzzle[key]
					if !ok {
						// TODO: default value?
						piece = newPathMergePuzzlePiece(UNKNOWN_PATH)
						puzzle[key] = piece
					}
					piece.Req[prefixPart] = struct{}{}
				}

				// add suffix req
				for x, suffixPart := range suffixParts {
					key := i + 1 + x
					piece, ok := puzzle[key]
					if !ok {
						// TODO: default value?
						piece = newPathMergePuzzlePiece(UNKNOWN_PATH)
						puzzle[key] = piece
					}
					piece.Req[suffixPart] = struct{}{}
				}

			} else {
				piece, ok := puzzle[i]
				if !ok {
					puzzle[i] = newPathMergePuzzlePiece(v)
				} else {
					piece.Val = v
				}
			}
		}
	}
	return puzzle
}

// Given 2 paths, see if we can resolve them into a single list
func MergeRoutePath(o []string, n []string) ([]string, error) {
	if len(o) != len(n) {
		return nil, fmt.Errorf("path lens don't match")
	}
	puzzle := CreatePuzzle(o, n)

	// resolve
	ret := make([]string, len(o))
	for i, piece := range puzzle {
		if !piece.Valid() {
			return nil, fmt.Errorf("doesn't fit")
		}
		ret[i] = piece.Val
	}
	FillPath(ret)
	return ret, nil

}

// TODO: tests for this
// take a list of strings, and replace all the "*" with the appropriate
// placeholder key
func FillPath(path []string) {
	missingPath := make([]int, 0)

	for i, hop := range path {
		if hop == "*" {
			missingPath = append(missingPath, i)
		}
	}
	// if there where any names missing (something in missingPath) then lets
	// make a unique name for this missing node.
	// Since we are just mapping, we don't know much about this node-- just
	// that is is between some other number of nodes. Because of this we'll
	// create the name of the node based on the surrounding nodes in the route
	// to avoid large numbers of duplicates (especially if the unknown node is
	// on either end of the route.
	// So if we have a route of: foo -> * -> * -> bar -> baz -> qux
	// the "*" nodes will end up with keys like:
	//		first "*": foo|*|*,bar
	// 		second "*": foo,*|*|,bar
	//
	// Note: the item surrounded by the "|" is the specific node we are looking at
	if len(missingPath) > 0 {
		namesToReplace := make(map[int]string)
		for _, i := range missingPath {
			prefixParts := make([]string, 0)
			suffixParts := make([]string, 0)
			// find the first path entry with a name before us
			for x := i - 1; x >= 0; x-- {
				// Prepend if it exists
				prefixParts = append([]string{path[x]}, prefixParts...)
				if path[x] != UNKNOWN_PATH {
					break
				}
			}
			// find the first path entry with a name after us
			for x := i + 1; x < len(path); x++ {
				suffixParts = append(suffixParts, path[x])
				if path[x] != UNKNOWN_PATH {
					break
				}
			}
			prefix := strings.Join(prefixParts, ",")
			suffix := strings.Join(suffixParts, ",")
			namesToReplace[i] = fmt.Sprintf("%s|%s|%s", prefix, UNKNOWN_PATH, suffix)
		}
		// replace all the names
		for i, newHopName := range namesToReplace {
			path[i] = newHopName
		}

	}

}
