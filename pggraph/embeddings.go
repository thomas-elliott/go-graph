package pggraph

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
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

// CreateVectorIndex builds the HNSW index VectorSearch uses, unless it exists.
func (s *Store) CreateVectorIndex(ctx context.Context, dims int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The 64MB default spills to disk and makes the full-set build much slower.
		if err := tx.Exec("SET LOCAL maintenance_work_mem = '2GB'").Error; err != nil {
			return err
		}
		return tx.Exec(fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS embeddings_hnsw_idx ON embeddings
			USING hnsw ((embedding::vector(%d)) vector_cosine_ops)`, dims)).Error
	})
}

// VectorSearch returns up to limit node IDs by cosine distance, approximately, using the HNSW index.
func (s *Store) VectorSearch(ctx context.Context, model string, query []float32, limit int) ([]Hit, error) {
	var hits []Hit
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// HNSW returns at most ef_search rows; twice the limit finds about 96% of the exact hits.
		if err := tx.Exec("SELECT set_config('hnsw.ef_search', ?, true)", fmt.Sprint(max(2*limit, 100))).Error; err != nil {
			return err
		}
		// The cast must match the index expression, or Postgres scans every row.
		return tx.Raw(fmt.Sprintf(`
			SELECT node_id AS id, 1 - (embedding::vector(%[1]d) <=> ?) AS rank
			FROM embeddings
			WHERE model = ?
			ORDER BY embedding::vector(%[1]d) <=> ?
			LIMIT ?`, len(query)), pgvector.NewVector(query), model, pgvector.NewVector(query), limit).Scan(&hits).Error
	})
	return hits, err
}

// ExactVectorSearch is VectorSearch without the index: slower, but the reference for what the index misses.
func (s *Store) ExactVectorSearch(ctx context.Context, model string, query []float32, limit int) ([]Hit, error) {
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

// VectorSearchWithin is VectorSearch limited to the given node IDs.
func (s *Store) VectorSearchWithin(ctx context.Context, model string, query []float32, ids []string, limit int) ([]Hit, error) {
	var hits []Hit
	err := s.db.WithContext(ctx).Raw(`
		SELECT node_id AS id, 1 - (embedding <=> ?) AS rank
		FROM embeddings
		WHERE model = ? AND node_id IN ?
		ORDER BY embedding <=> ?
		LIMIT ?`, pgvector.NewVector(query), model, ids, pgvector.NewVector(query), limit).Scan(&hits).Error
	return hits, err
}
