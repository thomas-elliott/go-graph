package main

import "github.com/thomas-elliott/go-graph/graph"

// loadGraph fills g with the nodes and edges from data/, splitting each
// node's sections into chunk nodes.
func loadGraph(g *graph.Memory) error {
	addNode := func(n nodeLine) error {
		g.AddNode(graph.Node{ID: n.ID, Labels: n.Labels, Properties: n.Properties})
		chunkNodes, chunkEdges := chunks(n)
		for _, c := range chunkNodes {
			g.AddNode(c)
		}
		for _, e := range chunkEdges {
			g.AddEdge(e)
		}
		return nil
	}

	addEdge := func(e edgeLine) error {
		g.AddEdge(graph.Edge{From: e.From, To: e.To, Type: e.Type, Properties: e.Properties})
		return nil
	}

	if err := loadJSONL(nodePath, addNode); err != nil {
		return err
	}
	return loadJSONL(edgePath, addEdge)
}
