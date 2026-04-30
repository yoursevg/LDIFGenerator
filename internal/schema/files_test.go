package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveInputPathsExpandsSchemaDirectories(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(dir, "a.ldif"),
		filepath.Join(dir, "b.schema"),
		filepath.Join(nested, "c.conf"),
	}
	for _, file := range files {
		if err := os.WriteFile(file, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveInputPaths([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, files) {
		t.Fatalf("paths = %#v, want %#v", got, files)
	}
}
