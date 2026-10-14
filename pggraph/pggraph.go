package pggraph

import (
	"context"
	_ "embed"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/thomas-elliott/go-graph/graph"
)

// The schema is manual, because gorm can't use ts_vector columns
//
//go:embed schema.sql
var schema string

type nodeRow struct {
	ID         string         `gorm:"primaryKey"`
	Labels     []string       `gorm:"serializer:json"`
	Properties map[string]any `gorm:"serializer:json"`
}

func (nodeRow) TableName() string { return "nodes" }

type edgeRow struct {
	// ID assigned by Postgres.
	ID         graph.EdgeID `gorm:"->"`
	From       string       `gorm:"column:from_id"`
	To         string       `gorm:"column:to_id"`
	Type       string
	Properties map[string]any `gorm:"serializer:json"`
}

func (edgeRow) TableName() string { return "edges" }

// Hit is a node ID matched by Search and Rank is its ts_rank score.
type Hit struct {
	ID   string
	Rank float64
}

// Store is a graph held in Postgres.
type Store struct {
	db *gorm.DB
}

var _ graph.Graph = (*Store)(nil)

// Open connects to the database and creates the schema if it is missing.
func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := db.WithContext(ctx).Exec(schema).Error; err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Clear deletes every node and edge.
func (s *Store) Clear(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec("TRUNCATE nodes, edges RESTART IDENTITY").Error
}

// AddNodes inserts nodes, overwriting any with the same ID, like graph.AddNode.
func (s *Store) AddNodes(ctx context.Context, nodes []graph.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	rows := make([]nodeRow, len(nodes))
	for i, n := range nodes {
		rows[i] = nodeRow{ID: n.ID, Labels: n.Labels, Properties: n.Properties}
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&rows).Error
}

// AddEdges inserts edges, skipping any that already exist, like graph.AddEdge.
// Both ends must already be stored.
func (s *Store) AddEdges(ctx context.Context, edges []graph.Edge) error {
	if len(edges) == 0 {
		return nil
	}
	rows := make([]edgeRow, len(edges))
	for i, e := range edges {
		rows[i] = edgeRow{From: e.From, To: e.To, Type: e.Type, Properties: e.Properties}
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

// Node implements graph.Graph.
func (s *Store) Node(ctx context.Context, id string) (graph.Node, bool, error) {
	var row nodeRow
	res := s.db.WithContext(ctx).Limit(1).Find(&row, "id = ?", id)
	if res.Error != nil || res.RowsAffected == 0 {
		return graph.Node{}, false, res.Error
	}
	return graph.Node{ID: row.ID, Labels: row.Labels, Properties: row.Properties}, true, nil
}

// Edge implements graph.Graph.
func (s *Store) Edge(ctx context.Context, id graph.EdgeID) (graph.Edge, bool, error) {
	var row edgeRow
	res := s.db.WithContext(ctx).Limit(1).Find(&row, "id = ?", id)
	if res.Error != nil || res.RowsAffected == 0 {
		return graph.Edge{}, false, res.Error
	}
	return graph.Edge{ID: row.ID, From: row.From, To: row.To, Type: row.Type, Properties: row.Properties}, true, nil
}

// edgesOf orders outgoing edges first, then by insertion, matching Memory.
// A self-loop is one row, so it is listed once.
const edgesOf = `
	FROM edges
	WHERE ((from_id = @id AND @out) OR (to_id = @id AND @in))
	  AND (@typ = '' OR type = @typ)
	ORDER BY from_id <> @id, id`

func edgeArgs(id string, dir graph.Direction, typ string) map[string]any {
	return map[string]any{"id": id, "out": dir != graph.In, "in": dir != graph.Out, "typ": typ}
}

// Edges implements graph.Graph.
func (s *Store) Edges(ctx context.Context, id string, dir graph.Direction, typ string) ([]graph.EdgeID, error) {
	var ids []graph.EdgeID
	err := s.db.WithContext(ctx).Raw("SELECT id"+edgesOf, edgeArgs(id, dir, typ)).Scan(&ids).Error
	return ids, err
}

// Neighbours implements graph.Graph.
func (s *Store) Neighbours(ctx context.Context, id string, dir graph.Direction, typ string) ([]string, error) {
	var ends []string
	err := s.db.WithContext(ctx).Raw(
		"SELECT CASE WHEN from_id = @id THEN to_id ELSE from_id END"+edgesOf,
		edgeArgs(id, dir, typ),
	).Scan(&ends).Error
	if err != nil {
		return nil, err
	}

	// Deduplicate here; DISTINCT would lose the edge order.
	seen := map[string]bool{}
	var ids []string
	for _, n := range ends {
		if seen[n] {
			continue
		}
		seen[n] = true
		ids = append(ids, n)
	}
	return ids, nil
}

// expandQuery is a breadth-first walk in one recursive query.
const expandQuery = `
WITH RECURSIVE walk(depth, frontier, visited) AS (
    -- CAST rather than ::text for GORM
    SELECT 0, ARRAY[CAST(@start AS text)], ARRAY[CAST(@start AS text)]
  UNION ALL
    SELECT w.depth + 1, next.nodes, w.visited || next.nodes
    FROM walk w
    CROSS JOIN LATERAL (
        SELECT array_agg(n ORDER BY first_seen) AS nodes
        FROM (
            SELECT n, min(seen) AS first_seen
            FROM (
                SELECT CASE WHEN e.from_id = f.node THEN e.to_id ELSE e.from_id END AS n,
                       row_number() OVER (ORDER BY f.ord, e.from_id <> f.node, e.id) AS seen
                FROM unnest(w.frontier) WITH ORDINALITY AS f(node, ord)
                JOIN edges e ON (e.from_id = f.node AND @out) OR (e.to_id = f.node AND @in)
                WHERE @typ = '' OR e.type = @typ
            ) candidates
            WHERE n <> ALL (w.visited)
            GROUP BY n
        ) found
    ) next
    -- End the walk early if frontier is empty or we reach k
    WHERE w.depth < @k AND cardinality(w.frontier) > 0
)
SELECT f.node AS id, w.depth
FROM walk w, unnest(w.frontier) WITH ORDINALITY AS f(node, ord)
ORDER BY w.depth, f.ord`

// Expand implements graph.Graph in a single recursive query.
func (s *Store) Expand(ctx context.Context, start string, k int, dir graph.Direction, typ string) ([]graph.Hop, error) {
	var hops []graph.Hop
	err := s.db.WithContext(ctx).Raw(expandQuery, map[string]any{
		"start": start, "k": k, "out": dir != graph.In, "in": dir != graph.Out, "typ": typ,
	}).Scan(&hops).Error
	return hops, err
}

// Search returns up to limit node IDs matching query, best results first.
// The query uses web search syntax: quoted phrases, "or", and -exclusions.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]Hit, error) {
	var hits []Hit
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, ts_rank(tsv, q) AS rank
		FROM nodes, websearch_to_tsquery('english', ?) q
		WHERE tsv @@ q
		ORDER BY rank DESC
		LIMIT ?`, query, limit).Scan(&hits).Error
	return hits, err
}

// Close closes the database connection.
func (s *Store) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}
