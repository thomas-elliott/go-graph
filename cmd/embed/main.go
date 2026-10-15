package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thomas-elliott/go-graph/embed"
	"github.com/thomas-elliott/go-graph/graph"
	"github.com/thomas-elliott/go-graph/pggraph"
)

const batchSize = 32

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	client, err := embed.FromEnv()
	if err != nil {
		return err
	}

	store, err := pggraph.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.CheckEmbeddings(ctx, client.Model, client.Dims); err != nil {
		return err
	}

	start := time.Now()
	done := 0
	after := ""
	for {
		nodes, err := store.Unembedded(ctx, after, batchSize)
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			break
		}

		ids := make([]string, len(nodes))
		texts := make([]string, len(nodes))
		for i, n := range nodes {
			ids[i] = n.ID
			texts[i] = embedText(n)
		}

		vectors, err := client.Embed(ctx, texts)
		if err != nil {
			return err
		}
		if err := store.SaveEmbeddings(ctx, client.Model, ids, vectors); err != nil {
			return err
		}

		done += len(nodes)
		after = nodes[len(nodes)-1].ID
		if done%(batchSize*100) == 0 {
			fmt.Printf("embedded %d nodes in %s\n", done, time.Since(start).Round(time.Second))
		}
	}
	fmt.Printf("embedded %d nodes in %s\n", done, time.Since(start).Round(time.Second))
	return nil
}

func embedText(n graph.Node) string {
	var parts []string
	for _, key := range []string{"dest", "heading", "name", "text", "content"} {
		if s, ok := n.Properties[key].(string); ok && s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}
