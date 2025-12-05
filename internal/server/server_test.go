package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDirListAndDeleteHandlers(t *testing.T) {
	root := t.TempDir()
	user := "tester"
	userRoot := filepath.Join(root, "users", user)
	if err := os.MkdirAll(filepath.Join(userRoot, "alpha", "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userRoot, "alpha", "beta", "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(root, user, "")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// list root
	resp, err := http.Get(ts.URL + "/api/v1/dir/list?path=.")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", resp.StatusCode)
	}
	var entries []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry at root, got %d", len(entries))
	}

	// delete recursively
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/dir?path=alpha&recursive=true", nil)
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status=%d", delResp.StatusCode)
	}

	// list root again -> empty
	resp2, err := http.Get(ts.URL + "/api/v1/dir/list?path=.")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var entries2 []map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&entries2); err != nil {
		t.Fatalf("decode2: %v", err)
	}
	if len(entries2) != 0 {
		t.Fatalf("want 0 entry at root, got %d", len(entries2))
	}
}

func TestStaticIndexServed(t *testing.T) {
	root := t.TempDir()
	s, err := NewServer(root, "default", "")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct == "" {
		t.Fatalf("expected content type for index.html")
	}
}

func TestShutdownStopsJanitor(t *testing.T) {
	root := t.TempDir()
	s, err := NewServer(root, "default", "")
	if err != nil {
		t.Fatal(err)
	}

	// Shutdown should stop the janitor and not block
	done := make(chan bool)
	go func() {
		s.Shutdown()
		done <- true
	}()

	select {
	case <-done:
		// Good, shutdown completed quickly
	case <-time.After(1 * time.Second):
		t.Fatal("shutdown should complete quickly")
	}

	// Multiple shutdown calls should be safe and not panic
	s.Shutdown()
	s.Shutdown()
}
