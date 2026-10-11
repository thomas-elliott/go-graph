package graph

import "context"

// Node is the basic data structure of the graph, storing data in Properties.
type Node struct {
	ID         string
	Properties map[string]any
	Labels     []string
}

// EdgeID is internally assigned.
type EdgeID int64

// Edge between two nodes (From and To). Type is optional, for filtering.
type Edge struct {
	ID         EdgeID
	From       string
	To         string
	Type       string
	Properties map[string]any
}

// Direction is used in searching/filtering to pick which edges to follow.
type Direction int

const (
	Out Direction = iota
	In
	Both
)

// Hop is a node ID found from a search and the depth from the start node.
type Hop struct {
	ID    string
	Depth int
}

// Graph is the common interface implemented by every backend
type Graph interface {
	// Node returns a node by its id and whether it exists.
	Node(ctx context.Context, id string) (Node, bool, error)
	// Edge returns an edge by its id and whether it exists.
	Edge(ctx context.Context, id EdgeID) (Edge, bool, error)
	// Edges returns a node's edge IDs, outgoing first. Empty typ matches any type.
	Edges(ctx context.Context, id string, dir Direction, typ string) ([]EdgeID, error)
	// Neighbours returns the unique adjacent node IDs, in Edges order.
	Neighbours(ctx context.Context, id string, dir Direction, typ string) ([]string, error)
	// Expand returns nodes within k hops of start in visit order, start at depth 0.
	Expand(ctx context.Context, start string, k int, dir Direction, typ string) ([]Hop, error)
}

// ExpandBFS implements Expand using only Neighbours.
func ExpandBFS(ctx context.Context, g Graph, start string, k int, dir Direction, typ string) ([]Hop, error) {
	visited := map[string]bool{start: true}
	frontier := []string{start}
	result := []Hop{{start, 0}}

	for depth := 1; depth <= k && len(frontier) > 0; depth++ {
		var newFrontier []string
		for _, id := range frontier {
			neighbours, err := g.Neighbours(ctx, id, dir, typ)
			if err != nil {
				return nil, err
			}
			for _, neighbour := range neighbours {
				if visited[neighbour] {
					continue
				}
				visited[neighbour] = true
				newFrontier = append(newFrontier, neighbour)
				result = append(result, Hop{neighbour, depth})
			}
		}
		frontier = newFrontier
	}

	return result, nil
}
