package storage

import (
    "archive/zip"
    "bytes"
    "io/fs"
    "os"
    "path/filepath"
    "testing"
)

func TestListAndDelete(t *testing.T) {
    root := t.TempDir()
    s := New(root)

    // create structure: a/, a/x.txt, b.txt
    if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(root, "a", "x.txt"), []byte("hi"), 0o644); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("bye"), 0o644); err != nil { t.Fatal(err) }

    entries, err := s.List(".")
    if err != nil { t.Fatalf("List root: %v", err) }
    if len(entries) != 2 { t.Fatalf("want 2 entries, got %d", len(entries)) }

    // delete file
    if err := s.Delete("b.txt", false); err != nil { t.Fatalf("delete file: %v", err) }
    // delete dir non-recursive should fail when not empty
    if err := s.Delete("a", false); err == nil {
        t.Fatalf("expected error deleting non-empty dir without recursive")
    }
    // recursive delete works
    if err := s.Delete("a", true); err != nil { t.Fatalf("delete dir recursive: %v", err) }
}

func TestZipPath_FileAndDir(t *testing.T) {
    root := t.TempDir()
    s := New(root)

    // Create files
    d := filepath.Join(root, "dir")
    if err := os.MkdirAll(d, 0o755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(d, "f1.txt"), []byte("one"), 0o644); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(d, "f2.txt"), []byte("two"), 0o644); err != nil { t.Fatal(err) }

    // Zip directory
    var buf bytes.Buffer
    zw := zip.NewWriter(&buf)
    if err := s.ZipPath(d, zw); err != nil { t.Fatalf("ZipPath dir: %v", err) }
    if err := zw.Close(); err != nil { t.Fatal(err) }

    // Ensure zip contains the files
    zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
    if err != nil { t.Fatal(err) }
    names := map[string]bool{}
    for _, f := range zr.File { names[f.Name] = true }
    // top-level folder name is the basename of d
    base := filepath.Base(d)
    if !names[base+"/f1.txt"] || !names[base+"/f2.txt"] {
        t.Fatalf("zip missing files: %v", names)
    }

    // Zip single file
    var buf2 bytes.Buffer
    zw2 := zip.NewWriter(&buf2)
    fpath := filepath.Join(d, "f1.txt")
    if err := s.ZipPath(fpath, zw2); err != nil { t.Fatalf("ZipPath file: %v", err) }
    if err := zw2.Close(); err != nil { t.Fatal(err) }
    zr2, err := zip.NewReader(bytes.NewReader(buf2.Bytes()), int64(buf2.Len()))
    if err != nil { t.Fatal(err) }
    if len(zr2.File) != 1 { t.Fatalf("want 1 file in zip, got %d", len(zr2.File)) }
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

// Ensure ZipPath respects file modes header creation (sanity check for no panic)
func TestZipPath_ModeHeader(t *testing.T) {
    root := t.TempDir()
    s := New(root)
    p := filepath.Join(root, "x")
    if err := os.WriteFile(p, []byte("x"), fs.FileMode(0o600)); err != nil { t.Fatal(err) }
    var buf bytes.Buffer
    zw := zip.NewWriter(&buf)
    if err := s.ZipPath(p, zw); err != nil { t.Fatalf("ZipPath: %v", err) }
    _ = zw.Close()
}

