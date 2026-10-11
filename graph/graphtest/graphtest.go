// Package graphtest holds the tests every graph.Graph backend must pass.
package graphtest

import (
	"fmt"
	"slices"
	"testing"

	"github.com/thomas-elliott/go-graph/graph"
)

// Build returns a backend holding only nodes and edges, added in that order.
type Build func(t *testing.T, nodes []graph.Node, edges []graph.Edge) graph.Graph

// Run tests the backend made by build.
func Run(t *testing.T, build Build) {
	t.Run("Node", func(t *testing.T) { testNode(t, build) })
	t.Run("Edges", func(t *testing.T) { testEdges(t, build) })
	t.Run("Neighbours", func(t *testing.T) { testNeighbours(t, build) })
	t.Run("Expand", func(t *testing.T) { testExpand(t, build) })
}

func nodes(ids ...string) []graph.Node {
	ns := make([]graph.Node, len(ids))
	for i, id := range ids {
		ns[i] = graph.Node{ID: id}
	}
	return ns
}

func testNode(t *testing.T, build Build) {
	ctx := t.Context()
	g := build(t, []graph.Node{{ID: "a", Labels: []string{"city"}, Properties: map[string]any{"name": "A"}}}, nil)

	n, ok, err := g.Node(ctx, "a")
	if err != nil || !ok || !slices.Equal(n.Labels, []string{"city"}) || n.Properties["name"] != "A" {
		t.Fatalf("Node(a) = %+v, %v, %v, want labels [city] and name A", n, ok, err)
	}
	if _, ok, err := g.Node(ctx, "missing"); ok || err != nil {
		t.Fatalf("Node(missing) = %v, %v, want false, nil", ok, err)
	}
	if _, ok, err := g.Edge(ctx, 999999); ok || err != nil {
		t.Fatalf("Edge(999999) = %v, %v, want false, nil", ok, err)
	}
}

// directionGraph has two a->b edges: Edges lists both, Neighbours lists b once.
func directionGraph(t *testing.T, build Build) graph.Graph {
	return build(t, nodes("a", "b", "c", "d", "x"), []graph.Edge{
		{From: "a", To: "b", Type: "part_of"},
		{From: "a", To: "b", Type: "link"},
		{From: "c", To: "b", Type: "part_of"},
		{From: "b", To: "d", Type: "part_of"},
		{From: "x", To: "x", Type: "loop"},
	})
}

// edgeKeys describes edges as "from->to type", since each backend assigns its own IDs.
func edgeKeys(t *testing.T, g graph.Graph, ids []graph.EdgeID) []string {
	keys := make([]string, len(ids))
	for i, id := range ids {
		e, ok, err := g.Edge(t.Context(), id)
		if err != nil || !ok {
			t.Fatalf("Edge(%v) = %v, %v", id, ok, err)
		}
		keys[i] = fmt.Sprintf("%s->%s %s", e.From, e.To, e.Type)
	}
	return keys
}

func testEdges(t *testing.T, build Build) {
	g := directionGraph(t, build)

	tests := []struct {
		name string
		id   string
		dir  graph.Direction
		typ  string
		want []string
	}{
		{"out", "b", graph.Out, "", []string{"b->d part_of"}},
		{"in", "b", graph.In, "", []string{"a->b part_of", "a->b link", "c->b part_of"}},
		{"in filtered", "b", graph.In, "part_of", []string{"a->b part_of", "c->b part_of"}},
		{"both, out first", "b", graph.Both, "part_of", []string{"b->d part_of", "a->b part_of", "c->b part_of"}},
		{"self-loop listed once", "x", graph.Both, "", []string{"x->x loop"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := g.Edges(t.Context(), tt.id, tt.dir, tt.typ)
			if err != nil {
				t.Fatal(err)
			}
			if got := edgeKeys(t, g, ids); !slices.Equal(got, tt.want) {
				t.Fatalf("Edges(%q, %v, %q) = %v, want %v", tt.id, tt.dir, tt.typ, got, tt.want)
			}
		})
	}
}

func testNeighbours(t *testing.T, build Build) {
	g := directionGraph(t, build)

	tests := []struct {
		name string
		id   string
		dir  graph.Direction
		typ  string
		want []string
	}{
		{"two edges, one neighbour", "a", graph.Out, "", []string{"b"}},
		{"in filtered", "b", graph.In, "part_of", []string{"a", "c"}},
		{"both, out first", "b", graph.Both, "", []string{"d", "a", "c"}},
		{"self-loop listed once", "x", graph.Both, "", []string{"x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.Neighbours(t.Context(), tt.id, tt.dir, tt.typ)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("Neighbours(%q, %v, %q) = %v, want %v", tt.id, tt.dir, tt.typ, got, tt.want)
			}
		})
	}
}

func testExpand(t *testing.T, build Build) {
	// a -> b -> c -> d -> f, with a shortcut a -> c and e -> a pointing in.
	g := build(t, nodes("a", "b", "c", "d", "e", "f"), []graph.Edge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "c", To: "d"},
		{From: "d", To: "f"},
		{From: "a", To: "c", Type: "part_of"},
		{From: "e", To: "a"},
	})

	tests := []struct {
		name string
		k    int
		dir  graph.Direction
		typ  string
		want []graph.Hop
	}{
		// c is reached at depth 1 by the shortcut, not 2 via b; f is 3 hops away so is left out.
		{"stops at k", 2, graph.Out, "", []graph.Hop{{ID: "a", Depth: 0}, {ID: "b", Depth: 1}, {ID: "c", Depth: 1}, {ID: "d", Depth: 2}}},
		{"in", 2, graph.In, "", []graph.Hop{{ID: "a", Depth: 0}, {ID: "e", Depth: 1}}},
		{"filtered", 3, graph.Out, "part_of", []graph.Hop{{ID: "a", Depth: 0}, {ID: "c", Depth: 1}}},
		{"k of zero is just the start", 0, graph.Both, "", []graph.Hop{{ID: "a", Depth: 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.Expand(t.Context(), "a", tt.k, tt.dir, tt.typ)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("Expand(a, %d, %v, %q) = %v, want %v", tt.k, tt.dir, tt.typ, got, tt.want)
			}
		})
	}
}
