package graph

import "slices"

// Node is the basic data structure of the graph, storing data in Properties.
type Node struct {
	ID         string
	Properties map[string]any
	Labels     []string
}

// EdgeID is internally assigned.
type EdgeID int

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

// Graph stores the whole graph data. Of note, it keeps track of assigned EdgeIDs and a fast lookup map
// for each node of their incoming and outgoing edges.
type Graph struct {
	nodes  map[string]Node
	edges  map[EdgeID]Edge
	nextID EdgeID
	out    map[string][]EdgeID
	in     map[string][]EdgeID
}

// New generates a new, empty graph.
func New() *Graph {
	return &Graph{
		nodes:  make(map[string]Node),
		edges:  make(map[EdgeID]Edge),
		nextID: 1,
		out:    make(map[string][]EdgeID),
		in:     make(map[string][]EdgeID),
	}
}

// AddNode will create, or overwrite a node with the given ID.
func (g *Graph) AddNode(n Node) {
	g.nodes[n.ID] = n
}

// Node returns a node by its id and whether it exists.
func (g *Graph) Node(id string) (Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

// AddEdge creates an edge between two nodes and returns its EdgeID.
// An existing edge will return the found EdgeID.
func (g *Graph) AddEdge(e Edge) EdgeID {
	if existing, ok := g.findEdge(e.From, e.To, e.Type); ok {
		return existing
	}

	e.ID = g.nextID
	g.edges[e.ID] = e
	g.out[e.From] = append(g.out[e.From], e.ID)
	g.in[e.To] = append(g.in[e.To], e.ID)
	g.nextID++

	return e.ID
}

// RemoveEdge deletes an edge by id and reports whether it existed.
func (g *Graph) RemoveEdge(id EdgeID) bool {
	e, ok := g.edges[id]
	if !ok {
		return false
	}

	g.out[e.From] = slices.DeleteFunc(g.out[e.From], func(x EdgeID) bool { return x == e.ID })
	g.in[e.To] = slices.DeleteFunc(g.in[e.To], func(x EdgeID) bool { return x == e.ID })
	delete(g.edges, id)
	return true
}

// Edge returns an edge by its id and indicates if any edge was found.
func (g *Graph) Edge(id EdgeID) (Edge, bool) {
	e, ok := g.edges[id]
	return e, ok
}

// Edges returns all edges of a given node, filtered by direction and type.
// Empty typ matches all edge types.
func (g *Graph) Edges(id string, dir Direction, typ string) []EdgeID {
	var edges []EdgeID
	switch dir {
	case Out:
		edges = g.out[id]
	case In:
		edges = g.in[id]
	case Both:
		edges = slices.Concat(g.out[id], g.in[id])
	}

	seen := map[EdgeID]bool{}
	var ids []EdgeID
	for _, e := range edges {
		if seen[e] {
			continue
		}
		seen[e] = true

		if typ == "" || g.edges[e].Type == typ {
			ids = append(ids, e)
		}
	}

	return ids
}

// Neighbours returns the unique node ids of all immediate neighbours of the node id filtered by
// direction and type. Empty typ matches all types.
func (g *Graph) Neighbours(id string, dir Direction, typ string) []string {
	seen := map[string]bool{}
	var ids []string
	for _, e := range g.Edges(id, dir, typ) {
		edge := g.edges[e]
		neighbour := edge.To
		if edge.To == id {
			neighbour = edge.From
		}

		if seen[neighbour] {
			continue
		}
		seen[neighbour] = true
		ids = append(ids, neighbour)
	}

	return ids
}

// Expand is a breadth-first walk up to k edges and returns nodes in order visited.
// Start is included at depth 0.
func (g *Graph) Expand(start string, k int, dir Direction, typ string) []Hop {
	visited := map[string]bool{start: true}
	frontier := []string{start}
	result := []Hop{{start, 0}}

	for depth := 1; depth <= k && len(frontier) > 0; depth++ {
		var newFrontier []string
		for _, id := range frontier {
			for _, neighbour := range g.Neighbours(id, dir, typ) {
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

	return result
}

// Len returns the count of nodes in the graph.
func (g *Graph) Len() int {
	return len(g.nodes)
}

func (g *Graph) findEdge(from, to, typ string) (EdgeID, bool) {
	for _, e := range g.out[from] {
		edge := g.edges[e]
		if edge.To == to && edge.Type == typ {
			return e, true
		}
	}
	return 0, false
}
