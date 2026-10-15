package pggraph

import (
	"os"
	"testing"

	"github.com/thomas-elliott/go-graph/graph"
	"github.com/thomas-elliott/go-graph/graph/graphtest"
)

func openTestStore(t *testing.T) *Store {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fail()
	}
	s, err := Open(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStore_Graph(t *testing.T) {
	s := openTestStore(t)
	graphtest.Run(t, func(t *testing.T, nodes []graph.Node, edges []graph.Edge) graph.Graph {
		ctx := t.Context()
		if err := s.Clear(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.AddNodes(ctx, nodes); err != nil {
			t.Fatal(err)
		}
		if err := s.AddEdges(ctx, edges); err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestStore_Search(t *testing.T) {
	s := openTestStore(t)
	ctx := t.Context()

	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	chunk := func(id, dest, heading, text string) graph.Node {
		return graph.Node{ID: id, Labels: []string{"chunk"}, Properties: map[string]any{"dest": dest, "heading": heading, "text": text}}
	}
	err := s.AddNodes(ctx, []graph.Node{
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

func TestStore_Embeddings(t *testing.T) {
	s := openTestStore(t)
	ctx := t.Context()

	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	err := s.AddNodes(ctx, []graph.Node{
		{ID: "Bruges", Labels: []string{"destination"}},
		{ID: "chunk:Bruges#0", Labels: []string{"chunk"}, Properties: map[string]any{"text": "a"}},
		{ID: "poi:Bruges#0", Labels: []string{"poi"}, Properties: map[string]any{"content": "b"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The destination has no text, so only the chunk and POI need vectors.
	nodes, err := s.Unembedded(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Properties["text"] != "a" {
		t.Fatalf("Unembedded = %+v, want the chunk and the POI", nodes)
	}

	if err := s.SaveEmbeddings(ctx, "m", []string{"chunk:Bruges#0"}, [][]float32{{1, 2}}); err != nil {
		t.Fatal(err)
	}
	nodes, err = s.Unembedded(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != "poi:Bruges#0" {
		t.Fatalf("Unembedded after saving = %+v, want only the POI", nodes)
	}

	if err := s.SaveEmbeddings(ctx, "m", []string{"poi:Bruges#0"}, [][]float32{{-1, 0}}); err != nil {
		t.Fatal(err)
	}
	hits, err := s.VectorSearch(ctx, "m", []float32{-1, 0.1}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].ID != "poi:Bruges#0" {
		t.Fatalf("VectorSearch = %+v, want the POI first", hits)
	}

	if err := s.CheckEmbeddings(ctx, "m", 2); err != nil {
		t.Errorf("same model and size: %v", err)
	}
	if err := s.CheckEmbeddings(ctx, "other", 2); err == nil {
		t.Error("different model: want an error")
	}
	if err := s.CheckEmbeddings(ctx, "m", 3); err == nil {
		t.Error("different size: want an error")
	}
}
