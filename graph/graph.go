package graph

type Node struct {
	ID         string
	Properties map[string]any
	Labels     []string
}

type Edge struct {
	From       string
	To         string
	Type       string
	Properties map[string]any
}

type Graph struct {
	nodes map[string]Node
	edges map[string][]Edge
}

func New() *Graph {
	return &Graph{
		nodes: make(map[string]Node),
		edges: make(map[string][]Edge),
	}
}

func (g *Graph) AddNode(n Node) {
	g.nodes[n.ID] = n
}

func (g *Graph) Node(id string) (Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

func (g *Graph) AddEdge(e Edge) {
	for _, existing := range g.edges[e.From] {
		if existing.To == e.To && existing.Type == e.Type {
			return
		}
	}
	g.edges[e.From] = append(g.edges[e.From], e)
}

func (g *Graph) Edges(id string) []Edge {
	return g.edges[id]
}

func (g *Graph) Neighbours(id string) []string {
	edges := g.edges[id]
	ids := make([]string, len(edges))
	for i, e := range edges {
		ids[i] = e.To
	}
	return ids
}

func (g *Graph) Len() int {
	return len(g.nodes)
}
