package main

import (
	"context"
	"fmt"
	"os"

	"github.com/thomas-elliott/go-graph/graph"
	"github.com/thomas-elliott/go-graph/pggraph"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	g := graph.New()
	if err := loadGraph(g); err != nil {
		return err
	}

	fmt.Printf("graph has %d nodes\n", g.Len())
	fmt.Printf("neighbours of Belgium: %v\n", g.Neighbours("Belgium", graph.Out, ""))
	fmt.Printf("neighbours of chunk:Bruges#0: %v\n", g.Neighbours("chunk:Bruges#0", graph.Out, ""))

	store, err := pggraph.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()

	if err := loadPostgres(ctx, store); err != nil {
		return err
	}

	hits, err := store.Search(ctx, "bruges trains", 5)
	if err != nil {
		return err
	}
	for _, h := range hits {
		fmt.Printf("  %.3f  %s\n", h.Rank, h.ID)
	}
	return nil
}
