package main

import (
	"fmt"

	"github.com/thomas-elliott/go-graph/graph"
)

func main() {
	g := graph.New()
	g.AddNode(graph.Node{ID: "a", Properties: map[string]any{"name": "Muriwai"}})
	g.AddNode(graph.Node{ID: "b", Properties: map[string]any{"name": "Waiwera"}})
	g.AddNode(graph.Node{ID: "c", Properties: map[string]any{"name": "Piha"}})

	g.AddEdge(graph.Edge{From: "a", To: "b"})
	g.AddEdge(graph.Edge{From: "a", To: "c", Properties: map[string]any{"sand": "black"}})

	fmt.Printf("graph has %d nodes\n", g.Len())
	fmt.Printf("neighbors of a: %v\n", g.Neighbors("a"))
}
