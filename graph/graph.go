package graph

import "slices"

type Node struct {
	ID         string
	Properties map[string]any
	Labels     []string
}

type EdgeID int

type Edge struct {
	ID         EdgeID
	From       string
	To         string
	Type       string
	Properties map[string]any
}

type Graph struct {
	nodes  map[string]Node
	edges  map[EdgeID]Edge
	nextID EdgeID
	out    map[string][]EdgeID
	in     map[string][]EdgeID
}

func New() *Graph {
	return &Graph{
		nodes:  make(map[string]Node),
		edges:  make(map[EdgeID]Edge),
		nextID: 1,
		out:    make(map[string][]EdgeID),
		in:     make(map[string][]EdgeID),
	}
}

func (g *Graph) AddNode(n Node) {
	g.nodes[n.ID] = n
}

func (g *Graph) Node(id string) (Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

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

func (g *Graph) Edges(id string) []EdgeID {
	return slices.Clone(g.out[id])
}

func (g *Graph) Neighbours(id string) []string {
	edges := g.out[id]
	ids := make([]string, len(edges))
	for i, e := range edges {
		edge := g.edges[e]
		ids[i] = edge.To
	}
	return ids
}

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
