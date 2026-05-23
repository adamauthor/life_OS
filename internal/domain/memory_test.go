package domain

import "testing"

func TestNewMemoryDefaultsAndNormalizes(t *testing.T) {
	memory, err := NewMemory(NewMemoryInput{
		RawText: "  идея: AI Life OS  ",
		Tags:    []string{" AI ", "ai", "", "Life"},
		Source:  "telegram",
	})
	if err != nil {
		t.Fatalf("NewMemory returned error: %v", err)
	}

	if memory.Type != MemoryTypeNote {
		t.Fatalf("Type = %q, want %q", memory.Type, MemoryTypeNote)
	}
	if memory.RawText != "идея: AI Life OS" {
		t.Fatalf("RawText = %q", memory.RawText)
	}
	if memory.Summary != "идея: AI Life OS" {
		t.Fatalf("Summary = %q", memory.Summary)
	}

	wantTags := []string{"ai", "life"}
	if len(memory.Tags) != len(wantTags) {
		t.Fatalf("Tags = %#v, want %#v", memory.Tags, wantTags)
	}
	for i := range wantTags {
		if memory.Tags[i] != wantTags[i] {
			t.Fatalf("Tags = %#v, want %#v", memory.Tags, wantTags)
		}
	}
}

func TestNewMemoryRequiresRawText(t *testing.T) {
	if _, err := NewMemory(NewMemoryInput{Source: "telegram"}); err == nil {
		t.Fatal("NewMemory returned nil error, want validation error")
	}
}
