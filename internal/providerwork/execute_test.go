package providerwork

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/felixlheureux/outpost/internal/codexadapter"
)

func TestProviderArtifactBoundAndUTF8Preview(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	writer := &eventFile{f: f}
	event := codexadapter.Event{Method: "item/completed", Params: json.RawMessage(`{"item":{"type":"agentMessage","text":"` + strings.Repeat("x", 16<<10) + `"}}`)}
	for i := 0; i < 700; i++ {
		if err := writer.record(event); err != nil {
			t.Fatal(err)
		}
	}
	if !writer.truncated || writer.written > 8<<20 {
		t.Fatalf("unbounded artifact: bytes=%d truncated=%v", writer.written, writer.truncated)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != writer.written {
		t.Fatalf("file size %d receipt %d", info.Size(), writer.written)
	}
	text := preview(strings.Repeat("é", 100), 35)
	if !utf8.ValidString(text) || !strings.Contains(text, "preview truncated") {
		t.Fatalf("invalid UTF-8 preview: %q", text)
	}
}
