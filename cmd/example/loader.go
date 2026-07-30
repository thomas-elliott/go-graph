package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/thomas-elliott/go-graph/graph"
)

type nodeLine struct {
	ID         string         `json:"id"`
	Labels     []string       `json:"labels"`
	Properties map[string]any `json:"properties"`
}

type edgeLine struct {
	From       string         `json:"from"`
	To         string         `json:"to"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
}

func loadSampleData(g *graph.Graph) error {
	nodePath := "./data/nodes.jsonl"
	edgePath := "./data/edges.jsonl"

	addNode := func(n nodeLine) {
		g.AddNode(graph.Node{ID: n.ID, Labels: n.Labels, Properties: n.Properties})
	}

	addEdge := func(e edgeLine) {
		g.AddEdge(graph.Edge{From: e.From, To: e.To, Type: e.Type, Properties: e.Properties})
	}

	if err := loadJSONL(nodePath, addNode); err != nil {
		return err
	}
	if err := loadJSONL(edgePath, addEdge); err != nil {
		return err
	}

	return nil
}

func loadJSONL[T any](filePath string, fn func(T)) error {
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
		fn(item)
	}

	return nil
}
