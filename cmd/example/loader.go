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

func loadSampleData(g *graph.Graph) error {
	nodePath := "./data/nodes.jsonl"
	edgePath := "./data/edges.jsonl"

	addNode := func(n nodeLine) {
		g.AddNode(graph.Node{ID: n.ID, Labels: n.Labels, Properties: n.Properties})
		addChunks(g, n.ID, n.Sections)
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

func addChunks(g *graph.Graph, parentID string, sections []sectionLine) {
	var previousID string
	for i, s := range sections {
		id := fmt.Sprintf("chunk:%s#%d", parentID, i)
		g.AddNode(graph.Node{
			ID:         id,
			Labels:     []string{"chunk"},
			Properties: map[string]any{"heading": s.Heading, "text": s.Text},
		})
		g.AddEdge(graph.Edge{From: parentID, To: id, Type: "has_chunk"})
		if i > 0 {
			g.AddEdge(graph.Edge{From: previousID, To: id, Type: "next_chunk"})
		}
		previousID = id
	}
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
