package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/thomas-elliott/go-graph/embed"
	"github.com/thomas-elliott/go-graph/pggraph"
)

const (
	pageSize  = 1000 // nodes per cache lookup and save
	batchSize = 8    // texts per embedding request
)

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
	done, fromCache := 0, 0
	after := ""
	for {
		nodes, err := store.Unembedded(ctx, after, pageSize)
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
			texts[i] = embed.Text(n)
		}

		vectors, hits, err := vectorsFor(ctx, client, store, texts)
		if err != nil {
			return err
		}
		if err := store.SaveEmbeddings(ctx, client.Model, ids, vectors); err != nil {
			return err
		}

		done += len(nodes)
		fromCache += hits
		after = nodes[len(nodes)-1].ID
		if done%(pageSize*10) == 0 {
			fmt.Printf("embedded %d nodes (%d from cache) in %s\n", done, fromCache, time.Since(start).Round(time.Second))
		}
	}
	fmt.Printf("embedded %d nodes (%d from cache) in %s\n", done, fromCache, time.Since(start).Round(time.Second))

	start = time.Now()
	if err := store.CreateVectorIndex(ctx, client.Dims); err != nil {
		return err
	}
	fmt.Printf("vector index ready in %s\n", time.Since(start).Round(time.Second))
	return nil
}

// vectorsFor will return a vector, first from cache if available, otherwise the embedding provider
func vectorsFor(ctx context.Context, client *embed.Client, store *pggraph.Store, texts []string) ([][]float32, int, error) {
	hashes := make([]string, len(texts))
	for i, t := range texts {
		hashes[i] = embed.Hash(t)
	}
	cached, err := store.CachedEmbeddings(ctx, client.Model, client.Dims, hashes)
	if err != nil {
		return nil, 0, err
	}

	var missingTexts, missingHashes []string
	for i, h := range hashes {
		if _, ok := cached[h]; !ok {
			missingTexts = append(missingTexts, texts[i])
			missingHashes = append(missingHashes, h)
		}
	}
	for start := 0; start < len(missingTexts); start += batchSize {
		end := min(start+batchSize, len(missingTexts))
		fetched, err := client.Embed(ctx, missingTexts[start:end])
		if err != nil {
			return nil, 0, err
		}
		if err := store.CacheEmbeddings(ctx, client.Model, client.Dims, missingHashes[start:end], fetched); err != nil {
			return nil, 0, err
		}
		for j, h := range missingHashes[start:end] {
			cached[h] = fetched[j]
		}
	}

	vectors := make([][]float32, len(texts))
	for i, h := range hashes {
		vectors[i] = cached[h]
	}
	return vectors, len(texts) - len(missingTexts), nil
}
