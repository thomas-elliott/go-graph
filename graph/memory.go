package graph

import (
	"context"
	"slices"
)

var _ Graph = (*Memory)(nil)

// Memory is an in-memory Graph that indexes each node's incoming and outgoing edges.
type Memory struct {
	nodes  map[string]Node
	edges  map[EdgeID]Edge
	nextID EdgeID
	out    map[string][]EdgeID
	in     map[string][]EdgeID
}

// NewMemory returns an empty Memory.
func NewMemory() *Memory {
	return &Memory{
		nodes:  make(map[string]Node),
		edges:  make(map[EdgeID]Edge),
		nextID: 1,
		out:    make(map[string][]EdgeID),
		in:     make(map[string][]EdgeID),
	}
}

// AddNode adds n, replacing any node with the same ID.
func (g *Memory) AddNode(n Node) {
	g.nodes[n.ID] = n
}

// Node implements Graph.
func (g *Memory) Node(_ context.Context, id string) (Node, bool, error) {
	n, ok := g.nodes[id]
	return n, ok, nil
}

// AddEdge adds e and returns its ID. If an edge with the same From, To and
// Type exists, it returns that edge's ID instead.
func (g *Memory) AddEdge(e Edge) EdgeID {
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
func (g *Memory) RemoveEdge(id EdgeID) bool {
	e, ok := g.edges[id]
	if !ok {
		return false
	}

	g.out[e.From] = slices.DeleteFunc(g.out[e.From], func(x EdgeID) bool { return x == e.ID })
	g.in[e.To] = slices.DeleteFunc(g.in[e.To], func(x EdgeID) bool { return x == e.ID })
	delete(g.edges, id)
	return true
}

// Edge implements Graph.
func (g *Memory) Edge(_ context.Context, id EdgeID) (Edge, bool, error) {
	e, ok := g.edges[id]
	return e, ok, nil
}

// Edges implements Graph.
func (g *Memory) Edges(_ context.Context, id string, dir Direction, typ string) ([]EdgeID, error) {
	return g.edgeIDs(id, dir, typ), nil
}

// edgeIDs is Edges without the context and error.
func (g *Memory) edgeIDs(id string, dir Direction, typ string) []EdgeID {
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

// Neighbours implements Graph.
func (g *Memory) Neighbours(_ context.Context, id string, dir Direction, typ string) ([]string, error) {
	seen := map[string]bool{}
	var ids []string
	for _, e := range g.edgeIDs(id, dir, typ) {
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

	return ids, nil
}

// Expand implements Graph.
func (g *Memory) Expand(ctx context.Context, start string, k int, dir Direction, typ string) ([]Hop, error) {
	return ExpandBFS(ctx, g, start, k, dir, typ)
}

// Len returns the number of nodes.
func (g *Memory) Len() int {
	return len(g.nodes)
}

func (g *Memory) findEdge(from, to, typ string) (EdgeID, bool) {
	for _, e := range g.out[from] {
		edge := g.edges[e]
		if edge.To == to && edge.Type == typ {
			return e, true
		}
	}
	return 0, false
}
