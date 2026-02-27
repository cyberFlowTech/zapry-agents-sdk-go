package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
)

// PgvectorStore implements agentsdk.EmbeddingStore using PostgreSQL + pgvector.
//
// Requires the pgvector extension: CREATE EXTENSION IF NOT EXISTS vector;
type PgvectorStore struct {
	db        *sql.DB
	table     string
	dimension int
}

// PgvectorConfig configures the pgvector store.
type PgvectorConfig struct {
	Table       string // table name, default "memory_vectors"
	Dimension   int    // vector dimension, default 1536 (OpenAI ada-002)
	AutoMigrate bool   // create table if not exist, default true
}

// NewPgvectorStore creates an EmbeddingStore backed by PostgreSQL + pgvector.
func NewPgvectorStore(db *sql.DB, config ...PgvectorConfig) (*PgvectorStore, error) {
	cfg := PgvectorConfig{Table: "memory_vectors", Dimension: 1536, AutoMigrate: true}
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.Table == "" {
		cfg.Table = "memory_vectors"
	}
	if cfg.Dimension <= 0 {
		cfg.Dimension = 1536
	}

	s := &PgvectorStore{db: db, table: cfg.Table, dimension: cfg.Dimension}
	if cfg.AutoMigrate {
		if err := s.migrate(); err != nil {
			return nil, fmt.Errorf("pgvector auto-migrate failed: %w", err)
		}
	}
	return s, nil
}

func (s *PgvectorStore) migrate() error {
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id        TEXT PRIMARY KEY,
		embedding vector(%d) NOT NULL,
		content   TEXT NOT NULL DEFAULT '',
		metadata  JSONB DEFAULT '{}',
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`, s.table, s.dimension)

	idx := fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS idx_%s_embedding ON %s USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)`,
		s.table, s.table,
	)

	if _, err := s.db.Exec(ddl); err != nil {
		return err
	}
	// Index creation may fail if not enough rows; ignore error
	s.db.Exec(idx)
	return nil
}

func (s *PgvectorStore) Upsert(ctx context.Context, id string, embedding []float32, content string, metadata map[string]string) error {
	vecStr := float32SliceToSQL(embedding)
	metaJSON, _ := json.Marshal(metadata)

	q := fmt.Sprintf(`INSERT INTO %s (id, embedding, content, metadata)
		VALUES ($1, $2::vector, $3, $4::jsonb)
		ON CONFLICT (id) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			content   = EXCLUDED.content,
			metadata  = EXCLUDED.metadata`, s.table)

	_, err := s.db.ExecContext(ctx, q, id, vecStr, content, string(metaJSON))
	return err
}

func (s *PgvectorStore) Search(ctx context.Context, query []float32, topK int, filter map[string]string) ([]agentsdk.MemoryHit, error) {
	vecStr := float32SliceToSQL(query)

	where := "1=1"
	var args []interface{}
	args = append(args, vecStr, topK)
	argIdx := 3

	for k, v := range filter {
		where += fmt.Sprintf(" AND metadata->>'%s' = $%d", k, argIdx)
		args = append(args, v)
		argIdx++
	}

	q := fmt.Sprintf(
		`SELECT id, content, metadata, 1 - (embedding <=> $1::vector) AS score
		 FROM %s WHERE %s
		 ORDER BY embedding <=> $1::vector
		 LIMIT $2`, s.table, where)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []agentsdk.MemoryHit
	for rows.Next() {
		var h agentsdk.MemoryHit
		var metaJSON string
		if err := rows.Scan(&h.ID, &h.Content, &metaJSON, &h.Score); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(metaJSON), &h.Metadata)
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

func (s *PgvectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", s.table, strings.Join(placeholders, ","))
	_, err := s.db.ExecContext(ctx, q, args...)
	return err
}

func (s *PgvectorStore) DeleteByMetadata(ctx context.Context, filter map[string]string) error {
	if len(filter) == 0 {
		return nil
	}
	where := make([]string, 0, len(filter))
	args := make([]interface{}, 0, len(filter))
	for k, v := range filter {
		where = append(where, fmt.Sprintf("metadata->>'%s' = $%d", k, len(args)+1))
		args = append(args, v)
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE %s", s.table, strings.Join(where, " AND "))
	_, err := s.db.ExecContext(ctx, q, args...)
	return err
}

func float32SliceToSQL(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// Compile-time interface check.
var _ agentsdk.EmbeddingStore = (*PgvectorStore)(nil)
