package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"life_os/internal/domain"
)

type MemoryRepository struct {
	db DB
}

type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewMemoryRepository(db DB) *MemoryRepository {
	return &MemoryRepository{db: db}
}

func (r *MemoryRepository) CreateMemory(ctx context.Context, memory domain.Memory) (domain.Memory, error) {
	metadata, err := json.Marshal(memory.Metadata)
	if err != nil {
		return domain.Memory{}, fmt.Errorf("marshal memory metadata: %w", err)
	}

	const query = `
		insert into memories (type, raw_text, summary, tags, source, embedding, metadata_json)
		values ($1, $2, $3, $4, $5, $6, $7::jsonb)
		returning id, created_at
	`

	var embedding any
	if len(memory.Embedding) > 0 {
		embedding = pgvector.NewVector(memory.Embedding)
	}

	if err := r.db.QueryRow(
		ctx,
		query,
		string(memory.Type),
		memory.RawText,
		memory.Summary,
		memory.Tags,
		memory.Source,
		embedding,
		string(metadata),
	).Scan(&memory.ID, &memory.CreatedAt); err != nil {
		return domain.Memory{}, fmt.Errorf("insert memory: %w", err)
	}

	return memory, nil
}

func (r *MemoryRepository) SearchMemories(ctx context.Context, queryEmbedding []float32, limit int) ([]domain.Memory, error) {
	if limit <= 0 {
		limit = 6
	}
	const query = `
		select id, type, raw_text, summary, tags, source, created_at, metadata_json
		from memories
		where embedding is not null
		order by embedding <-> $1
		limit $2
	`

	rows, err := r.query(ctx, query, pgvector.NewVector(queryEmbedding), limit)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var memories []domain.Memory
	for rows.Next() {
		var memory domain.Memory
		var memoryType string
		var metadataBytes []byte
		if err := rows.Scan(
			&memory.ID,
			&memoryType,
			&memory.RawText,
			&memory.Summary,
			&memory.Tags,
			&memory.Source,
			&memory.CreatedAt,
			&metadataBytes,
		); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memory.Type = domain.MemoryType(memoryType)
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &memory.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal memory metadata: %w", err)
			}
		}
		memories = append(memories, memory)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memories: %w", err)
	}
	return memories, nil
}

func (r *MemoryRepository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	type queryer interface {
		Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	}
	db, ok := r.db.(queryer)
	if !ok {
		return nil, fmt.Errorf("db does not support Query")
	}
	return db.Query(ctx, sql, args...)
}
