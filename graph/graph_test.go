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

	got := g.Neighbours("a", Out, "")
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("neighbours(a) = %v, want [b c]", got)
	}
	edges := g.Edges("a", Out, "")
	if len(edges) != 2 {
		t.Fatalf("Edges(a) = %v, want 2 edges", edges)
	}
	if e, ok := g.Edge(edges[1]); !ok || e.Properties["sand"] != "black" {
		t.Fatalf("Edge(%v) = %+v, %v, want sand=black", edges[1], e, ok)
	}
	if _, ok := g.Edge(999); ok {
		t.Fatal("Edge(999) reported present")
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

	got := g.Neighbours("a", Out, "")
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("neighbours(a) = %v, want [b c]", got)
	}

	e, ok := g.findEdge("a", "b", "ToDelete")
	if !ok {
		t.Fatalf("findEdge(\"a\", \"b\") = false, want true")
	}
	g.RemoveEdge(e)

	got = g.Neighbours("a", Out, "")
	if !slices.Equal(got, []string{"c"}) {
		t.Fatalf("neighbours(a) = %v, want [c]", got)
	}

	newE := g.AddEdge(Edge{From: "a", To: "b", Type: "ToDelete"})
	if e == newE {
		t.Fatalf("AddEdge(\"a\", \"b\", \"ToDelete\") returned same ID")
	}

	got = g.Neighbours("a", Out, "")
	if !slices.Equal(got, []string{"c", "b"}) {
		t.Fatalf("neighbours(a) = %v, want [c b]", got)
	}
}

// directionGraph has two edges a->b of different types
func directionGraph() (*Graph, map[string]EdgeID) {
	g := New()
	ids := map[string]EdgeID{
		"a-b part_of": g.AddEdge(Edge{From: "a", To: "b", Type: "part_of"}),
		"a-b link":    g.AddEdge(Edge{From: "a", To: "b", Type: "link"}),
		"c-b part_of": g.AddEdge(Edge{From: "c", To: "b", Type: "part_of"}),
		"b-d part_of": g.AddEdge(Edge{From: "b", To: "d", Type: "part_of"}),
		"x-x loop":    g.AddEdge(Edge{From: "x", To: "x", Type: "loop"}),
	}
	return g, ids
}

func TestGraph_EdgesDirection(t *testing.T) {
	g, ids := directionGraph()

	tests := []struct {
		name string
		id   string
		dir  Direction
		typ  string
		want []EdgeID
	}{
		{"out", "b", Out, "", []EdgeID{ids["b-d part_of"]}},
		{"in", "b", In, "", []EdgeID{ids["a-b part_of"], ids["a-b link"], ids["c-b part_of"]}},
		{"in filtered", "b", In, "part_of", []EdgeID{ids["a-b part_of"], ids["c-b part_of"]}},
		{"both, out first", "b", Both, "part_of", []EdgeID{ids["b-d part_of"], ids["a-b part_of"], ids["c-b part_of"]}},
		{"self-loop listed once", "x", Both, "", []EdgeID{ids["x-x loop"]}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.Edges(tt.id, tt.dir, tt.typ); !slices.Equal(got, tt.want) {
				t.Fatalf("Edges(%q, %v, %q) = %v, want %v", tt.id, tt.dir, tt.typ, got, tt.want)
			}
		})
	}
}

func TestGraph_NeighboursDirection(t *testing.T) {
	g, _ := directionGraph()

	tests := []struct {
		name string
		id   string
		dir  Direction
		typ  string
		want []string
	}{
		{"two edges, one neighbour", "a", Out, "", []string{"b"}},
		{"in filtered", "b", In, "part_of", []string{"a", "c"}},
		{"both, out first", "b", Both, "", []string{"d", "a", "c"}},
		{"self-loop listed once", "x", Both, "", []string{"x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.Neighbours(tt.id, tt.dir, tt.typ); !slices.Equal(got, tt.want) {
				t.Fatalf("Neighbours(%q, %v, %q) = %v, want %v", tt.id, tt.dir, tt.typ, got, tt.want)
			}
		})
	}
}

func TestGraph_Expand(t *testing.T) {
	// a -> b -> c -> d -> f, with a shortcut a -> c and e -> a pointing in.
	g := New()
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "b", To: "c"})
	g.AddEdge(Edge{From: "c", To: "d"})
	g.AddEdge(Edge{From: "d", To: "f"})
	g.AddEdge(Edge{From: "a", To: "c", Type: "part_of"})
	g.AddEdge(Edge{From: "e", To: "a"})

	tests := []struct {
		name string
		k    int
		dir  Direction
		typ  string
		want []Hop
	}{
		// c is reached at depth 1 by the shortcut, not 2 via b; f is 3 hops away so is left out.
		{"stops at k", 2, Out, "", []Hop{{"a", 0}, {"b", 1}, {"c", 1}, {"d", 2}}},
		{"in", 2, In, "", []Hop{{"a", 0}, {"e", 1}}},
		{"filtered", 3, Out, "part_of", []Hop{{"a", 0}, {"c", 1}}},
		{"k of zero is just the start", 0, Both, "", []Hop{{"a", 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.Expand("a", tt.k, tt.dir, tt.typ); !slices.Equal(got, tt.want) {
				t.Fatalf("Expand(a, %d, %v, %q) = %v, want %v", tt.k, tt.dir, tt.typ, got, tt.want)
			}
		})
	}
}
