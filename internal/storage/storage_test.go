package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func TestDownloadZip_EscapesSlashBranch(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	ctx := context.Background()

	branch := "feature/sub"
	dest := filepath.Join(root, "out.zip")

	var seenPath string
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		seenPath = req.URL.EscapedPath()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("zipdata")),
			Header:     make(http.Header),
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })

	if err := s.downloadZip(ctx, "owner/repo", branch, "", dest); err != nil {
		t.Fatalf("downloadZip: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "zipdata" {
		t.Fatalf("unexpected content: %q", string(data))
	}
	if !strings.Contains(seenPath, "feature%2Fsub") {
		t.Fatalf("expected escaped branch in path, got %q", seenPath)
	}
}

func TestFetchBranchSHA_EscapesSlashBranch(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	ctx := context.Background()

	branch := "feature/sub"
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.EscapedPath(), "feature%2Fsub") {
			return nil, fmt.Errorf("path not escaped: %s", req.URL.EscapedPath())
		}
		body := `{"commit":{"sha":"abc123"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })

	sha, err := s.fetchBranchSHA(ctx, "owner/repo", branch, "")
	if err != nil {
		t.Fatalf("fetchBranchSHA: %v", err)
	}
	if sha != "abc123" {
		t.Fatalf("unexpected sha: %s", sha)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
