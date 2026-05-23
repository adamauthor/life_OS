package storage

import (
	"context"
	"os"
	"testing"

	"life_os/internal/domain"
)

func TestMemoryRepositoryCreateMemoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	postgres, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgres returned error: %v", err)
	}
	t.Cleanup(postgres.Close)

	repository := NewMemoryRepository(postgres.Pool)
	memory, err := domain.NewMemory(domain.NewMemoryInput{
		UserID:    domain.UserIDFromTelegram(123),
		RawText:   "integration memory",
		Tags:      []string{"test", "memory"},
		Source:    "telegram",
		Embedding: make([]float32, 1536),
		Metadata: map[string]any{
			"chat_id": float64(123),
		},
	})
	if err != nil {
		t.Fatalf("NewMemory returned error: %v", err)
	}

	saved, err := repository.CreateMemory(ctx, memory)
	if err != nil {
		t.Fatalf("CreateMemory returned error: %v", err)
	}
	t.Cleanup(func() {
		if _, err := postgres.Pool.Exec(ctx, "delete from memories where id = $1", saved.ID); err != nil {
			t.Fatalf("cleanup memory: %v", err)
		}
	})

	if saved.ID == 0 {
		t.Fatal("saved.ID = 0, want generated id")
	}
	if saved.CreatedAt.IsZero() {
		t.Fatal("saved.CreatedAt is zero")
	}

	found, err := repository.SearchMemories(ctx, memory.UserID, make([]float32, 1536), 1)
	if err != nil {
		t.Fatalf("SearchMemories returned error: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("SearchMemories returned no rows")
	}
}
