package output

import (
	"strings"
	"testing"
)

func TestStoreCreateAndReadAll(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	entry, writer, err := store.Create()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := writer.Write([]byte("full output")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := store.ReadAll(entry.ID)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "full output" {
		t.Fatalf("ReadAll() = %q, want %q", data, "full output")
	}
}

func TestStoreRejectsInvalidID(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	_, err = store.ReadAll("../secret")
	if err == nil {
		t.Fatal("ReadAll() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid output_id") {
		t.Fatalf("ReadAll() error = %q, want invalid output_id", err)
	}
}
