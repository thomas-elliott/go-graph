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
	From       string `gorm:"column:from_id"`
	To         string `gorm:"column:to_id"`
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
