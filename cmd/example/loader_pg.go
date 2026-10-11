package main

import (
	"context"

	"github.com/thomas-elliott/go-graph/graph"
	"github.com/thomas-elliott/go-graph/pggraph"
)

const batchSize = 1000

// loadPostgres replaces the full graph in postgres with data from data/
func loadPostgres(ctx context.Context, s *pggraph.Store) error {
	if err := s.Clear(ctx); err != nil {
		return err
	}

	var nodes []graph.Node
	var edges []graph.Edge

	// Order matters, nodes have to go first
	flush := func() error {
		if err := s.AddNodes(ctx, nodes); err != nil {
			return err
		}
		if err := s.AddEdges(ctx, edges); err != nil {
			return err
		}
		nodes, edges = nodes[:0], edges[:0]
		return nil
	}

	addNode := func(n nodeLine) error {
		chunkNodes, chunkEdges := chunks(n)
		nodes = append(nodes, graph.Node{ID: n.ID, Labels: n.Labels, Properties: n.Properties})
		nodes = append(nodes, chunkNodes...)
		edges = append(edges, chunkEdges...)
		if len(nodes) >= batchSize {
			return flush()
		}
		return nil
	}

	addEdge := func(e edgeLine) error {
		edges = append(edges, graph.Edge{From: e.From, To: e.To, Type: e.Type, Properties: e.Properties})
		if len(edges) >= batchSize {
			return flush()
		}
		return nil
	}

	if err := loadJSONL(nodePath, addNode); err != nil {
		return err
	}
	if err := loadJSONL(edgePath, addEdge); err != nil {
		return err
	}
	return flush()
}
