package graph_test

import (
	"testing"

	"github.com/thomas-elliott/go-graph/graph"
	"github.com/thomas-elliott/go-graph/graph/graphtest"
)

func TestMemory_Graph(t *testing.T) {
	graphtest.Run(t, func(t *testing.T, nodes []graph.Node, edges []graph.Edge) graph.Graph {
		g := graph.NewMemory()
		for _, n := range nodes {
			g.AddNode(n)
		}
		for _, e := range edges {
			g.AddEdge(e)
		}
		return g
	})
}
