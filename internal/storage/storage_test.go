package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListAndDelete(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	// create structure: a/, a/x.txt, b.txt
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "x.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("bye"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := s.List(".")
	if err != nil {
		t.Fatalf("List root: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}

	// delete file
	if err := s.Delete("b.txt", false); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	// delete dir non-recursive should fail when not empty
	if err := s.Delete("a", false); err == nil {
		t.Fatalf("expected error deleting non-empty dir without recursive")
	}
	// recursive delete works
	if err := s.Delete("a", true); err != nil {
		t.Fatalf("delete dir recursive: %v", err)
	}
}

func TestList_NotFound(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if _, err := s.List("nope"); err == nil {
		t.Fatalf("expected error for not found path")
	}
}

func TestSafeJoinPreventsEscape(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	// Accessing parent should fail via public API like Delete/List
	if err := s.Delete("..", true); err == nil {
		t.Fatalf("expected error for escaping path")
	}
	if _, err := s.List("../outside"); err == nil {
		t.Fatalf("expected error for escaping list")
	}
}
