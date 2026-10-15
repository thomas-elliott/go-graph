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

const batchSize = 1000

func main() {
	dir := flag.String("data", "./data", "folder holding nodes.jsonl and edges.jsonl")
	flag.Parse()

	if err := run(context.Background(), *dir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dir string) error {
	store, err := pggraph.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.Clear(ctx); err != nil {
		return err
	}

	var nodes []graph.Node
	var edges []graph.Edge

	// Order matters, nodes have to go first
	flush := func() error {
		if err := store.AddNodes(ctx, nodes); err != nil {
			return err
		}
		if err := store.AddEdges(ctx, edges); err != nil {
			return err
		}
		nodes, edges = nodes[:0], edges[:0]
		return nil
	}

	addNode := func(n graph.Node) error {
		nodes = append(nodes, n)
		if len(nodes) >= batchSize {
			return flush()
		}
		return nil
	}

	addEdge := func(e graph.Edge) error {
		edges = append(edges, e)
		if len(edges) >= batchSize {
			return flush()
		}
		return nil
	}

	if err := dataset.Read(dir, addNode, addEdge); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}
	fmt.Printf("loaded %s into postgres\n", dir)
	return nil
}
