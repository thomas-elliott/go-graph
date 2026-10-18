package main

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/thomas-elliott/go-graph/graph"
	"github.com/thomas-elliott/go-graph/pggraph"
)

// walkConfig is to configure variations of a walk.
type walkConfig struct {
	seeds         int
	minSimilarity float64
	links         bool
	fuse          bool
	rank          string
}

// walkMethod seeds a walk with vector hits, then ranks the nodes it reaches.
func walkMethod(store *pggraph.Store, model string, embedQuery func(context.Context, string) ([]float32, error), cfg walkConfig, limit int) (method, error) {
	if !slices.Contains([]string{"similarity", "decay", "pagerank"}, cfg.rank) {
		return nil, fmt.Errorf("unknown rank %q", cfg.rank)
	}

	return func(ctx context.Context, query string) ([]string, error) {
		v, err := embedQuery(ctx, query)
		if err != nil {
			return nil, err
		}
		seeds, err := store.VectorSearch(ctx, model, v, cfg.seeds)
		if err != nil {
			return nil, err
		}

		candidates := map[string]bool{}
		var walked []pggraph.Hit
		var reached []map[string]int
		for _, seed := range seeds {
			candidates[seed.ID] = true
			if seed.Rank < cfg.minSimilarity {
				continue
			}
			hops, err := walkFrom(ctx, store, seed.ID, cfg.links)
			if err != nil {
				return nil, err
			}
			walked = append(walked, seed)
			reached = append(reached, hops)
			for id := range hops {
				candidates[id] = true
			}
		}

		ids := slices.Collect(maps.Keys(candidates))
		similar, err := store.VectorSearchWithin(ctx, model, v, ids, len(ids))
		if err != nil {
			return nil, err
		}
		var ranked []string
		switch cfg.rank {
		case "similarity":
			ranked = hitIDs(similar)
		case "decay":
			ranked = sortByScore(similar, decay(walked, reached))
		case "pagerank":
			edges, err := store.EdgesAmong(ctx, ids)
			if err != nil {
				return nil, err
			}
			ranked = sortByScore(similar, pageRank(edges, walked))
		}
		ranked = ranked[:min(limit, len(ranked))]
		if !cfg.fuse {
			return ranked, nil
		}

		plain, err := store.VectorSearch(ctx, model, v, limit)
		if err != nil {
			return nil, err
		}
		fused := fuse(ranked, hitIDs(plain))
		return fused[:min(limit, len(fused))], nil
	}, nil
}

// walkFrom returns the nodes reached from a seed with their hop counts:
// its destination, that destination's part_of parent, the destinations
// part_of it, with links the destinations it links to, and their chunks and POIs.
func walkFrom(ctx context.Context, g graph.Graph, seed string, links bool) (map[string]int, error) {
	hops := map[string]int{seed: 0}
	reach := func(depth int, ids []string) {
		for _, id := range ids {
			if _, ok := hops[id]; !ok {
				hops[id] = depth
			}
		}
	}

	dests, err := neighboursOfTypes(ctx, g, seed, graph.In, "has_chunk", "has_poi")
	if err != nil {
		return nil, err
	}
	reach(1, dests)

	regionTypes := []string{"part_of"}
	var region []string
	for _, dest := range dests {
		children, err := neighboursOfTypes(ctx, g, dest, graph.Out, "has_chunk", "has_poi")
		if err != nil {
			return nil, err
		}
		reach(2, children)

		up, err := neighboursOfTypes(ctx, g, dest, graph.Out, regionTypes...)
		if err != nil {
			return nil, err
		}
		down, err := neighboursOfTypes(ctx, g, dest, graph.In, regionTypes...)
		if err != nil {
			return nil, err
		}
		region = append(region, up...)
		region = append(region, down...)
		if links {
			linked, err := neighboursOfTypes(ctx, g, dest, graph.Out, "link")
			if err != nil {
				return nil, err
			}
			region = append(region, linked...)
		}
	}
	reach(2, region)

	for _, dest := range region {
		children, err := neighboursOfTypes(ctx, g, dest, graph.Out, "has_chunk", "has_poi")
		if err != nil {
			return nil, err
		}
		reach(3, children)
	}
	return hops, nil
}

func neighboursOfTypes(ctx context.Context, g graph.Graph, id string, dir graph.Direction, types ...string) ([]string, error) {
	var all []string
	for _, typ := range types {
		ids, err := g.Neighbours(ctx, id, dir, typ)
		if err != nil {
			return nil, err
		}
		all = append(all, ids...)
	}
	return all, nil
}

// sortByScore orders hits by score, highest first; hits are already in
// similarity order, which breaks ties.
func sortByScore(hits []pggraph.Hit, score map[string]float64) []string {
	ids := hitIDs(hits)
	slices.SortStableFunc(ids, func(a, b string) int {
		return cmp.Compare(score[b], score[a])
	})
	return ids
}

// decay scores a node by each seed's similarity, halved per hop from that seed.
func decay(walked []pggraph.Hit, reached []map[string]int) map[string]float64 {
	score := map[string]float64{}
	for i, seed := range walked {
		for id, hops := range reached[i] {
			score[id] += seed.Rank * math.Pow(0.5, float64(hops))
		}
	}
	return score
}

// pageRank is personalised PageRank over the edges as undirected links,
// restarting at the seeds in proportion to their similarity.
func pageRank(edges []graph.Edge, walked []pggraph.Hit) map[string]float64 {
	const restart, rounds = 0.15, 20

	neighbours := map[string][]string{}
	for _, e := range edges {
		neighbours[e.From] = append(neighbours[e.From], e.To)
		neighbours[e.To] = append(neighbours[e.To], e.From)
	}

	var total float64
	for _, seed := range walked {
		total += seed.Rank
	}
	start := map[string]float64{}
	for _, seed := range walked {
		start[seed.ID] += seed.Rank / total
	}

	score := maps.Clone(start)
	for range rounds {
		next := map[string]float64{}
		for id, p := range start {
			next[id] += restart * p
		}
		for id, s := range score {
			ns := neighbours[id]
			if len(ns) == 0 {
				for seed, p := range start {
					next[seed] += (1 - restart) * s * p
				}
				continue
			}
			for _, n := range ns {
				next[n] += (1 - restart) * s / float64(len(ns))
			}
		}
		score = next
	}
	return score
}
