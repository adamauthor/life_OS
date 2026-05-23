package ai

import "testing"

func TestAudioFileMetadata(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		wantFilename    string
		wantContentType string
	}{
		{name: "empty", filename: "", wantFilename: "voice.ogg", wantContentType: "audio/ogg"},
		{name: "ogg", filename: "voice.ogg", wantFilename: "voice.ogg", wantContentType: "audio/ogg"},
		{name: "oga", filename: "voice.oga", wantFilename: "voice.oga", wantContentType: "audio/ogg"},
		{name: "webm", filename: "voice.webm", wantFilename: "voice.webm", wantContentType: "audio/webm"},
		{name: "unknown", filename: "voice", wantFilename: "voice.ogg", wantContentType: "audio/ogg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFilename, gotContentType := audioFileMetadata(tt.filename)
			if gotFilename != tt.wantFilename {
				t.Fatalf("filename = %q, want %q", gotFilename, tt.wantFilename)
			}
			if gotContentType != tt.wantContentType {
				t.Fatalf("contentType = %q, want %q", gotContentType, tt.wantContentType)
			}
		})
	}
}
