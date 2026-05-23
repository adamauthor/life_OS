package domain

import (
	"errors"
	"strings"
	"time"
)

type MemoryType string

const (
	MemoryTypeIdea       MemoryType = "idea"
	MemoryTypeTask       MemoryType = "task"
	MemoryTypeNote       MemoryType = "note"
	MemoryTypeReflection MemoryType = "reflection"
	MemoryTypeEvent      MemoryType = "event"
	MemoryTypeQuestion   MemoryType = "question"
)

type Memory struct {
	ID        int64
	Type      MemoryType
	RawText   string
	Summary   string
	Tags      []string
	Source    string
	CreatedAt time.Time
	Embedding []float32
	Metadata  map[string]any
}

type NewMemoryInput struct {
	Type      MemoryType
	RawText   string
	Summary   string
	Tags      []string
	Source    string
	Embedding []float32
	Metadata  map[string]any
}

func NewMemory(input NewMemoryInput) (Memory, error) {
	rawText := strings.TrimSpace(input.RawText)
	if rawText == "" {
		return Memory{}, errors.New("memory raw text is required")
	}

	memoryType := input.Type
	if memoryType == "" {
		memoryType = MemoryTypeNote
	}
	if !memoryType.Valid() {
		return Memory{}, errors.New("memory type is invalid")
	}

	source := strings.TrimSpace(input.Source)
	if source == "" {
		return Memory{}, errors.New("memory source is required")
	}

	summary := strings.TrimSpace(input.Summary)
	if summary == "" {
		summary = rawText
	}

	return Memory{
		Type:      memoryType,
		RawText:   rawText,
		Summary:   summary,
		Tags:      normalizeTags(input.Tags),
		Source:    source,
		Embedding: input.Embedding,
		Metadata:  input.Metadata,
	}, nil
}

func (t MemoryType) Valid() bool {
	switch t {
	case MemoryTypeIdea, MemoryTypeTask, MemoryTypeNote, MemoryTypeReflection, MemoryTypeEvent, MemoryTypeQuestion:
		return true
	default:
		return false
	}
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}
