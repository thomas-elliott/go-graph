package graph

import (
	"slices"
	"testing"
)

func TestGraph_Neighbours(t *testing.T) {
	g := New()
	g.AddNode(Node{ID: "a"})
	g.AddNode(Node{ID: "b"})
	g.AddNode(Node{ID: "c"})
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "a", To: "c", Properties: map[string]any{"sand": "black"}})
	g.AddEdge(Edge{From: "a", To: "b"}) // duplicate, ignored

	got := g.Neighbours("a")
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("neighbours(a) = %v, want [b c]", got)
	}
	if edges := g.Edges("a"); len(edges) != 2 || g.edges[edges[1]].Properties["sand"] != "black" {
		t.Fatalf("Edges(a) = %+v, want 2 edges with sand=black on the second", edges)
	}
	if g.Len() != 3 {
		t.Fatalf("Len = %d, want 3", g.Len())
	}
	if _, ok := g.Node("missing"); ok {
		t.Fatal("Node(missing) reported present")
	}
}

func TestGraph_RemoveEdge(t *testing.T) {
	g := New()
	g.AddNode(Node{ID: "a"})
	g.AddNode(Node{ID: "b"})
	g.AddNode(Node{ID: "c"})
	g.AddEdge(Edge{From: "a", To: "b", Type: "ToDelete"})
	g.AddEdge(Edge{From: "a", To: "c"})

	got := g.Neighbours("a")
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("neighbours(a) = %v, want [b c]", got)
	}

	e, ok := g.findEdge("a", "b", "ToDelete")
	if !ok {
		t.Fatalf("findEdge(\"a\", \"b\") = false, want true")
	}
	g.RemoveEdge(e)

	got = g.Neighbours("a")
	if !slices.Equal(got, []string{"c"}) {
		t.Fatalf("neighbours(a) = %v, want [c]", got)
	}

	newE := g.AddEdge(Edge{From: "a", To: "b", Type: "ToDelete"})
	if e == newE {
		t.Fatalf("AddEdge(\"a\", \"b\", \"ToDelete\") returned same ID")
	}

	got = g.Neighbours("a")
	if !slices.Equal(got, []string{"c", "b"}) {
		t.Fatalf("neighbours(a) = %v, want [c b]", got)
	}
}
