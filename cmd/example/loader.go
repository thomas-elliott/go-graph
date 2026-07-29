package main

import (
	"bufio"
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

	nodeFile, err := os.Open(nodePath)
	if err != nil {
		return fmt.Errorf("failed to open nodes file: %w", err)
	}

	edgeFile, err := os.Open("./data/edges.jsonl")
	if err != nil {
		nodeFile.Close()
		return fmt.Errorf("failed to open edges file: %w", err)
	}

	if err := loadNodes(g, nodeFile); err != nil {
		edgeFile.Close()
		return err
	}

	if err := loadEdges(g, edgeFile); err != nil {
		return err
	}

	nodeFile.Close()
	return nil
}

func loadNodes(g *graph.Graph, f *os.File) error {
	sc := bufio.NewScanner(f)

	for sc.Scan() {
		var n nodeLine
		if err := json.Unmarshal(sc.Bytes(), &n); err != nil {
			return err
		}
		g.AddNode(graph.Node{ID: n.ID, Labels: n.Labels, Properties: n.Properties})
	}

	return sc.Err()
}

func loadEdges(g *graph.Graph, f *os.File) error {
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e edgeLine
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return err
		}

		g.AddEdge(graph.Edge{From: e.From, To: e.To, Type: e.Type, Properties: e.Properties})
	}

	return sc.Err()
}
