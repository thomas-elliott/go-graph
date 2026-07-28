package graph

import (
	"slices"
	"testing"
)

func TestNeighbors(t *testing.T) {
	g := New()
	g.AddNode(Node{ID: "a"})
	g.AddNode(Node{ID: "b"})
	g.AddNode(Node{ID: "c"})
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "a", To: "c", Properties: map[string]any{"sand": "black"}})
	g.AddEdge(Edge{From: "a", To: "b"}) // duplicate, ignored

	got := g.Neighbors("a")
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("neighbors(a) = %v, want [b c]", got)
	}
	if edges := g.Edges("a"); len(edges) != 2 || edges[1].Properties["sand"] != "black" {
		t.Fatalf("Edges(a) = %+v, want 2 edges with sand=black on the second", edges)
	}
	if g.Len() != 3 {
		t.Fatalf("Len = %d, want 3", g.Len())
	}
	if _, ok := g.Node("missing"); ok {
		t.Fatal("Node(missing) reported present")
	}
}
