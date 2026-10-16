package pggraph

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm/clause"

	"github.com/thomas-elliott/go-graph/graph"
)

type embeddingRow struct {
	NodeID    string `gorm:"primaryKey"`
	Model     string
	Embedding pgvector.Vector
}

func (embeddingRow) TableName() string { return "embeddings" }

// CheckEmbeddings validates model and size metadata from the DB.
func (s *Store) CheckEmbeddings(ctx context.Context, model string, dims int) error {
	var other []embeddingRow
	err := s.db.WithContext(ctx).
		Select("model").
		Where("model <> ? OR vector_dims(embedding) <> ?", model, dims).
		Limit(1).Find(&other).Error
	if err != nil {
		return err
	}
	if len(other) > 0 {
		return fmt.Errorf("embeddings already hold vectors from %q or another size", other[0].Model)
	}
	return nil
}

// Unembedded returns the next set of nodes that need embedding.
func (s *Store) Unembedded(ctx context.Context, after string, limit int) ([]graph.Node, error) {
	var rows []nodeRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, labels, properties
		FROM nodes n
		WHERE id > ?
		  AND (properties->>'text' IS NOT NULL OR properties->>'content' IS NOT NULL)
		  AND NOT EXISTS (SELECT 1 FROM embeddings e WHERE e.node_id = n.id)
		ORDER BY id
		LIMIT ?`, after, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	nodes := make([]graph.Node, len(rows))
	for i, r := range rows {
		nodes[i] = graph.Node{ID: r.ID, Labels: r.Labels, Properties: r.Properties}
	}
	return nodes, nil
}

// SaveEmbeddings stores one vector per node ID.
func (s *Store) SaveEmbeddings(ctx context.Context, model string, ids []string, vectors [][]float32) error {
	if len(ids) == 0 {
		return nil
	}
	rows := make([]embeddingRow, len(ids))
	for i, id := range ids {
		rows[i] = embeddingRow{NodeID: id, Model: model, Embedding: pgvector.NewVector(vectors[i])}
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&rows).Error
}

// VectorSearch returns up to limit node IDs, by cosine distance.
func (s *Store) VectorSearch(ctx context.Context, model string, query []float32, limit int) ([]Hit, error) {
	var hits []Hit
	err := s.db.WithContext(ctx).Raw(`
		SELECT node_id AS id, 1 - (embedding <=> ?) AS rank
		FROM embeddings
		WHERE model = ?
		ORDER BY embedding <=> ?
		LIMIT ?`, pgvector.NewVector(query), model, pgvector.NewVector(query), limit).Scan(&hits).Error
	return hits, err
}

type cacheRow struct {
	Model     string `gorm:"primaryKey"`
	Dims      int    `gorm:"primaryKey"`
	TextHash  string `gorm:"primaryKey"`
	Embedding pgvector.Vector
}

func (cacheRow) TableName() string { return "embedding_cache" }

// CachedEmbeddings returns the cached vectors keyed by their content hash.
func (s *Store) CachedEmbeddings(ctx context.Context, model string, dims int, hashes []string) (map[string][]float32, error) {
	var rows []cacheRow
	err := s.db.WithContext(ctx).
		Where("model = ? AND dims = ? AND text_hash IN ?", model, dims, hashes).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	cached := make(map[string][]float32, len(rows))
	for _, r := range rows {
		cached[r.TextHash] = r.Embedding.Slice()
	}
	return cached, nil
}

// CacheEmbeddings stores one vector per text hash.
func (s *Store) CacheEmbeddings(ctx context.Context, model string, dims int, hashes []string, vectors [][]float32) error {
	if len(hashes) == 0 {
		return nil
	}
	rows := make([]cacheRow, len(hashes))
	for i, h := range hashes {
		rows[i] = cacheRow{Model: model, Dims: dims, TextHash: h, Embedding: pgvector.NewVector(vectors[i])}
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}
