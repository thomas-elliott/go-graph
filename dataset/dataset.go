package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

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

// Read streams nodes.jsonl and edges.jsonl from dir, so the whole file is
// never held in memory. Each node's sections become chunk nodes.
func Read(dir string, addNode func(graph.Node) error, addEdge func(graph.Edge) error) error {
	readNode := func(n nodeLine) error {
		if err := addNode(graph.Node{ID: n.ID, Labels: n.Labels, Properties: n.Properties}); err != nil {
			return err
		}
		chunkNodes, chunkEdges := chunks(n)
		for _, c := range chunkNodes {
			if err := addNode(c); err != nil {
				return err
			}
		}
		for _, e := range chunkEdges {
			if err := addEdge(e); err != nil {
				return err
			}
		}
		return nil
	}

	readEdge := func(e edgeLine) error {
		return addEdge(graph.Edge{From: e.From, To: e.To, Type: e.Type, Properties: e.Properties})
	}

	if err := readJSONL(filepath.Join(dir, "nodes.jsonl"), readNode); err != nil {
		return err
	}
	return readJSONL(filepath.Join(dir, "edges.jsonl"), readEdge)
}

// maxChunkChars is about 1,000 tokens at 4 characters per token.
const maxChunkChars = 4000

// chunks turns a node's sections into chunk nodes, linked to the parent by
// has_chunk and to each other by next_chunk. Sections over maxChunkChars are
// split into pieces with IDs #i.0, #i.1, so unsplit chunks keep their IDs.
func chunks(n nodeLine) ([]graph.Node, []graph.Edge) {
	var nodes []graph.Node
	var edges []graph.Edge
	var previousID string
	for i, s := range n.Sections {
		pieces := splitText(s.Text, maxChunkChars)
		for j, text := range pieces {
			id := fmt.Sprintf("chunk:%s#%d", n.ID, i)
			if len(pieces) > 1 {
				id = fmt.Sprintf("%s.%d", id, j)
			}
			nodes = append(nodes, graph.Node{
				ID:     id,
				Labels: []string{"chunk"},
				// dest matches the POI property, so search can treat both alike.
				Properties: map[string]any{"dest": n.ID, "heading": s.Heading, "text": text},
			})
			edges = append(edges, graph.Edge{From: n.ID, To: id, Type: "has_chunk"})
			if previousID != "" {
				edges = append(edges, graph.Edge{From: previousID, To: id, Type: "next_chunk"})
			}
			previousID = id
		}
	}
	return nodes, edges
}

// splitText packs text into pieces of at most max characters.
func splitText(text string, max int) []string {
	if utf8.RuneCountInString(text) <= max {
		return []string{text}
	}

	var units []string
	for _, line := range strings.SplitAfter(text, "\n") {
		for _, sentence := range splitOversized(line, ". ", max) {
			units = append(units, splitOversized(sentence, " ", max)...)
		}
	}

	var pieces []string
	var current strings.Builder
	for _, u := range units {
		if current.Len() > 0 && utf8.RuneCountInString(current.String())+utf8.RuneCountInString(u) > max {
			pieces = append(pieces, strings.TrimSpace(current.String()))
			current.Reset()
		}
		current.WriteString(u)
	}
	if strings.TrimSpace(current.String()) != "" {
		pieces = append(pieces, strings.TrimSpace(current.String()))
	}
	return pieces
}

// splitOversized splits s after each sep, but only if s is over max.
func splitOversized(s, sep string, max int) []string {
	if utf8.RuneCountInString(s) <= max {
		return []string{s}
	}
	return strings.SplitAfter(s, sep)
}

func readJSONL[T any](filePath string, fn func(T) error) error {
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
