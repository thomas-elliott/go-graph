package pggraph

import (
	"context"
	"os"
	"testing"

	"github.com/thomas-elliott/go-graph/graph"
)

func TestStore_Search(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()

	s, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	chunk := func(id, dest, heading, text string) graph.Node {
		return graph.Node{ID: id, Labels: []string{"chunk"}, Properties: map[string]any{"dest": dest, "heading": heading, "text": text}}
	}
	err = s.AddNodes(ctx, []graph.Node{
		{ID: "Bruges", Labels: []string{"destination"}},
		chunk("chunk:Bruges#3", "Bruges", "Get in", "Trains run every half hour from Brussels."),
		chunk("chunk:Ghent#3", "Ghent", "Get in", "Trains run often from Brussels."),
	})
	if err != nil {
		t.Fatal(err)
	}
	edges := []graph.Edge{{From: "Bruges", To: "chunk:Bruges#3", Type: "has_chunk"}}
	if err := s.AddEdges(ctx, append(edges, edges...)); err != nil { // duplicate is skipped
		t.Fatal(err)
	}

	hits, err := s.Search(ctx, "bruges trains", 10)
	if err != nil {
		t.Fatal(err)
	}
	// Only Bruges's chunk has both words, and only because dest is indexed.
	if len(hits) != 1 || hits[0].ID != "chunk:Bruges#3" {
		t.Fatalf("Search(bruges trains) = %+v, want only chunk:Bruges#3", hits)
	}
}
