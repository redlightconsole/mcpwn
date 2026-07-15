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

func TestStoreReadRange(t *testing.T) {
	store, id := writeStoredOutput(t, "0123456789")

	result, err := store.Read(id, 2, 4)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(result.Data) != "2345" {
		t.Fatalf("Read() data = %q, want %q", result.Data, "2345")
	}
	if result.NextOffset != 6 {
		t.Fatalf("Read() NextOffset = %d, want 6", result.NextOffset)
	}
	if result.EOF {
		t.Fatal("Read() EOF = true, want false")
	}
}

func TestStoreTail(t *testing.T) {
	store, id := writeStoredOutput(t, "one\ntwo\nthree\n")

	result, err := store.Tail(id, 2)
	if err != nil {
		t.Fatalf("Tail() error = %v", err)
	}
	if result != "two\nthree" {
		t.Fatalf("Tail() = %q, want %q", result, "two\nthree")
	}
}

func TestStoreSearch(t *testing.T) {
	store, id := writeStoredOutput(t, "alpha\nbeta\nalphabet\n")

	matches, err := store.Search(id, "alpha", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("len(Search()) = %d, want 2", len(matches))
	}
	if matches[0].Line != 1 || matches[1].Line != 3 {
		t.Fatalf("Search() lines = %d,%d; want 1,3", matches[0].Line, matches[1].Line)
	}
}

func writeStoredOutput(t *testing.T, content string) (*Store, string) {
	t.Helper()

	store, err := NewStore(t.TempDir())
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

	return store, entry.ID
}
