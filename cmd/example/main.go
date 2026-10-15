package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/thomas-elliott/go-graph/dataset"
	"github.com/thomas-elliott/go-graph/embed"
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

	if err := search(ctx, store, "bruges trains"); err != nil {
		return err
	}
	if err := search(ctx, store, "how do I get to Bruges by rail"); err != nil {
		return err
	}

	if err := describe(ctx, "memory", memory); err != nil {
		return err
	}
	return describe(ctx, "postgres", store)
}

func search(ctx context.Context, store *pggraph.Store, query string) error {
	hits, err := store.Search(ctx, query, 5)
	if err != nil {
		return err
	}
	printHits("text search: "+query, hits)

	client, err := embed.FromEnv()
	if err != nil {
		fmt.Println("vector search skipped:", err)
		return nil
	}
	vectors, err := client.Embed(ctx, []string{query})
	if err != nil {
		return err
	}
	hits, err = store.VectorSearch(ctx, client.Model, vectors[0], 5)
	if err != nil {
		return err
	}
	printHits("vector search: "+query, hits)
	return nil
}

func printHits(title string, hits []pggraph.Hit) {
	fmt.Println(title)
	for _, h := range hits {
		fmt.Printf("  %.3f  %s\n", h.Rank, h.ID)
	}
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
