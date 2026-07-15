package server

import (
	"reflect"
	"strings"
	"testing"

	"mcpwn/internal/config"
	"mcpwn/internal/output"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuildArgs(t *testing.T) {
	tool := &config.Tool{
		FixedArgs: []string{"scan"},
		Args: []config.Arg{
			{Name: "target", Required: true, Positional: true},
			{Name: "ports", Flag: "-p"},
			{Name: "verbose", Flag: "-v", Type: "boolean"},
			{Name: "dry_run", Flag: "--dry-run", Type: "boolean"},
			{Name: "extra_args", Flag: ""},
		},
	}

	args, err := buildArgs(tool, map[string]interface{}{
		"target":     "127.0.0.1",
		"ports":      "80,443",
		"verbose":    true,
		"dry_run":    false,
		"extra_args": "--reason --top-ports 10",
	})
	if err != nil {
		t.Fatalf("buildArgs() error = %v", err)
	}

	want := []string{"scan", "-p", "80,443", "-v", "--reason", "--top-ports", "10", "127.0.0.1"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("buildArgs() = %#v, want %#v", args, want)
	}
}

func TestBuildArgsMissingRequired(t *testing.T) {
	tool := &config.Tool{
		Args: []config.Arg{{Name: "target", Required: true}},
	}

	_, err := buildArgs(tool, map[string]interface{}{})
	if err == nil {
		t.Fatal("buildArgs() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Fatalf("buildArgs() error = %q, want it to mention target", err)
	}
}

func TestHandleReadOutput(t *testing.T) {
	outputID := writeServerOutput(t, "abcdef")

	result, err := (&MCPServer{}).handleReadOutput(map[string]interface{}{
		"output_id": outputID,
		"offset":    float64(2),
		"limit":     float64(3),
	})
	if err != nil {
		t.Fatalf("handleReadOutput() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("handleReadOutput() IsError = true: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.HasPrefix(text, "cde\n[INFO] offset=2 next_offset=5 size=6 eof=false") {
		t.Fatalf("handleReadOutput() = %q", text)
	}
}

func TestHandleTailOutput(t *testing.T) {
	outputID := writeServerOutput(t, "one\ntwo\nthree\n")

	result, err := (&MCPServer{}).handleTailOutput(map[string]interface{}{
		"output_id": outputID,
		"lines":     float64(2),
	})
	if err != nil {
		t.Fatalf("handleTailOutput() error = %v", err)
	}
	if resultText(t, result) != "two\nthree" {
		t.Fatalf("handleTailOutput() = %q, want %q", resultText(t, result), "two\nthree")
	}
}

func TestHandleSearchOutput(t *testing.T) {
	outputID := writeServerOutput(t, "alpha\nbeta\nalphabet\n")

	result, err := (&MCPServer{}).handleSearchOutput(map[string]interface{}{
		"output_id":   outputID,
		"query":       "alpha",
		"max_matches": float64(1),
	})
	if err != nil {
		t.Fatalf("handleSearchOutput() error = %v", err)
	}
	if resultText(t, result) != "1:alpha" {
		t.Fatalf("handleSearchOutput() = %q, want %q", resultText(t, result), "1:alpha")
	}
}

func TestHandleOutputRejectsInvalidLimit(t *testing.T) {
	result, err := (&MCPServer{}).handleReadOutput(map[string]interface{}{
		"output_id": "test",
		"limit":     float64(maxReadLimit + 1),
	})
	if err != nil {
		t.Fatalf("handleReadOutput() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("handleReadOutput() IsError = false, want true")
	}
}

func writeServerOutput(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("MCPWN_OUTPUT_DIR", dir)
	store, err := output.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	entry, writer, err := store.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return entry.ID
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("len(result.Content) = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("result.Content[0] = %T, want *mcp.TextContent", result.Content[0])
	}
	return text.Text
}
