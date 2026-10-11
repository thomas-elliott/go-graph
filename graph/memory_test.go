package graph

import (
	"slices"
	"testing"
)

func TestMemory_Neighbours(t *testing.T) {
	g := NewMemory()
	ctx := t.Context()
	g.AddNode(Node{ID: "a"})
	g.AddNode(Node{ID: "b"})
	g.AddNode(Node{ID: "c"})
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "a", To: "c", Properties: map[string]any{"sand": "black"}})
	g.AddEdge(Edge{From: "a", To: "b"}) // duplicate, ignored

	got, _ := g.Neighbours(ctx, "a", Out, "")
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("neighbours(a) = %v, want [b c]", got)
	}
	edges, _ := g.Edges(ctx, "a", Out, "")
	if len(edges) != 2 {
		t.Fatalf("Edges(a) = %v, want 2 edges", edges)
	}
	if e, ok, _ := g.Edge(ctx, edges[1]); !ok || e.Properties["sand"] != "black" {
		t.Fatalf("Edge(%v) = %+v, %v, want sand=black", edges[1], e, ok)
	}
	if g.Len() != 3 {
		t.Fatalf("Len = %d, want 3", g.Len())
	}
}

func TestMemory_RemoveEdge(t *testing.T) {
	g := NewMemory()
	ctx := t.Context()
	g.AddNode(Node{ID: "a"})
	g.AddNode(Node{ID: "b"})
	g.AddNode(Node{ID: "c"})
	g.AddEdge(Edge{From: "a", To: "b", Type: "ToDelete"})
	g.AddEdge(Edge{From: "a", To: "c"})

	got, _ := g.Neighbours(ctx, "a", Out, "")
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("neighbours(a) = %v, want [b c]", got)
	}

	e, ok := g.findEdge("a", "b", "ToDelete")
	if !ok {
		t.Fatalf("findEdge(\"a\", \"b\") = false, want true")
	}
	g.RemoveEdge(e)

	got, _ = g.Neighbours(ctx, "a", Out, "")
	if !slices.Equal(got, []string{"c"}) {
		t.Fatalf("neighbours(a) = %v, want [c]", got)
	}

	newE := g.AddEdge(Edge{From: "a", To: "b", Type: "ToDelete"})
	if e == newE {
		t.Fatalf("AddEdge(\"a\", \"b\", \"ToDelete\") returned same ID")
	}

	got, _ = g.Neighbours(ctx, "a", Out, "")
	if !slices.Equal(got, []string{"c", "b"}) {
		t.Fatalf("neighbours(a) = %v, want [c b]", got)
	}
}
