package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSONStoreWriteUsesPrivatePermissions(t *testing.T) {
	t.Parallel()

	type sample struct {
		Name string `json:"name"`
	}

	path := filepath.Join(t.TempDir(), "state.json")
	store := NewJSONStore[sample](path, nil)

	if err := store.Write(sample{Name: "demo"}); err != nil {
		t.Fatalf("write state: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", info.Mode().Perm())
	}

	value, err := store.Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if value.Name != "demo" {
		t.Fatalf("expected stored value to round-trip, got %#v", value)
	}
}
