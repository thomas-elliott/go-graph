package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/thomas-elliott/go-graph/graph"
)

const (
	nodePath = "./data/nodes.jsonl"
	edgePath = "./data/edges.jsonl"
)

type nodeLine struct {
	ID         string         `json:"id"`
	Labels     []string       `json:"labels"`
	Properties map[string]any `json:"properties"`
	Sections   []sectionLine  `json:"sections"`
}

type sectionLine struct {
	Heading string `json:"heading"`
	Text    string `json:"text"`
}

type edgeLine struct {
	From       string         `json:"from"`
	To         string         `json:"to"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
}

// chunks turns a node's sections into chunk nodes, linked to the parent by
// has_chunk and to each other by next_chunk. Both loaders use it so they build
// the same graph.
func chunks(n nodeLine) ([]graph.Node, []graph.Edge) {
	var nodes []graph.Node
	var edges []graph.Edge
	var previousID string
	for i, s := range n.Sections {
		id := fmt.Sprintf("chunk:%s#%d", n.ID, i)
		nodes = append(nodes, graph.Node{
			ID:     id,
			Labels: []string{"chunk"},
			// dest matches the POI property, so search can treat both alike.
			Properties: map[string]any{"dest": n.ID, "heading": s.Heading, "text": s.Text},
		})
		edges = append(edges, graph.Edge{From: n.ID, To: id, Type: "has_chunk"})
		if i > 0 {
			edges = append(edges, graph.Edge{From: previousID, To: id, Type: "next_chunk"})
		}
		previousID = id
	}
	return nodes, edges
}

func loadJSONL[T any](filePath string, fn func(T) error) error {
	f, err := os.Open(filePath)

	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)

	for decoder.More() {
		var item T
		if err := decoder.Decode(&item); err != nil {
			return fmt.Errorf("decode %s: %w", filePath, err)
		}
		if err := fn(item); err != nil {
			return err
		}
	}

	return nil
}
