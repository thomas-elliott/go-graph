package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/thomas-elliott/go-graph/dataset"
	"github.com/thomas-elliott/go-graph/graph"
	"github.com/thomas-elliott/go-graph/pggraph"
)

func main() {
	dir := flag.String("data", "./data", "data folder")
	flag.Parse()

	if err := run(context.Background(), *dir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dir string) error {
	memory := graph.NewMemory()
	addNode := func(n graph.Node) error { memory.AddNode(n); return nil }
	addEdge := func(e graph.Edge) error { memory.AddEdge(e); return nil }
	if err := dataset.Read(dir, addNode, addEdge); err != nil {
		return err
	}
	fmt.Printf("memory graph has %d nodes\n", memory.Len())

	store, err := pggraph.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()

	hits, err := store.Search(ctx, "bruges trains", 5)
	if err != nil {
		return err
	}
	fmt.Println("search: bruges trains")
	for _, h := range hits {
		fmt.Printf("  %.3f  %s\n", h.Rank, h.ID)
	}

	if err := describe(ctx, "memory", memory); err != nil {
		return err
	}
	return describe(ctx, "postgres", store)
}

// describe runs the same queries on any backend.
func describe(ctx context.Context, name string, g graph.Graph) error {
	parts, err := g.Neighbours(ctx, "Belgium", graph.In, "part_of")
	if err != nil {
		return err
	}
	fmt.Printf("%s: part of Belgium: %v\n", name, parts)

	// The top search hit plus the chunks either side of it.
	around, err := g.Expand(ctx, "chunk:Bruges#2", 1, graph.Both, "next_chunk")
	if err != nil {
		return err
	}
	fmt.Printf("%s: around chunk:Bruges#2: %v\n", name, around)
	return nil
}
