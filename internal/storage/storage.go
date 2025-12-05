package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrBadPath  = errors.New("bad path")
	ErrNotFound = errors.New("not found")
)

type Storage struct {
	Root string

	mu   sync.Mutex
	lock map[string]*sync.Mutex
}

func New(root string) *Storage { return &Storage{Root: root} }

// EnsureRepo ensures a cached repo (owner/repo) at branch exists under workspace.
// If missing, it downloads from GitHub zipball and extracts into
//
//	<root>/users/<user>/repos/<owner>/<repo>/<branch>
//
// If branch is empty, uses "default" path name.
func (s *Storage) EnsureRepo(ctx context.Context, user, ownerRepo, branch, token string) (string, error) {
	user = strings.Trim(user, "/ ")
	if user == "" {
		user = "default"
	}
	if strings.ContainsRune(user, '/') || strings.ContainsRune(user, '\\') {
		return "", fmt.Errorf("invalid user: %w", ErrBadPath)
	}
	ownerRepo = strings.Trim(ownerRepo, "/")
	if ownerRepo == "" || strings.Count(ownerRepo, "/") != 1 {
		return "", fmt.Errorf("owner/repo expected: %w", ErrBadPath)
	}
	if branch == "" {
		branch = "default"
	}
	dir := filepath.Join(s.Root, "users", user, "repos", ownerRepo, branch)
	unlock := s.acquire(user, ownerRepo, branch)
	defer unlock()

	// Fast path: already present
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, nil
	}

	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp(parent, ".tmp-download-*")
	if err != nil {
		return "", err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	if err := s.downloadAndExtract(ctx, ownerRepo, branch, token, tmpDir); err != nil {
		cleanup()
		return "", err
	}
	if err := os.Rename(tmpDir, dir); err != nil {
		cleanup()
		return "", err
	}
	_ = s.touchDir(dir)
	return dir, nil
}

// List lists entries under the given relative path.
func (s *Storage) List(rel string) ([]Entry, error) {
	abs, err := s.safeJoin(rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	result := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Name() == ".last_access" {
			continue
		}
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		p := filepath.ToSlash(filepath.Join(rel, e.Name()))
		result = append(result, Entry{
			Name:  e.Name(),
			Path:  p,
			IsDir: e.IsDir(),
			Size:  size,
		})
	}
	return result, nil
}

// Delete removes the relative path. If recursive is false and path is a directory, it must be empty.
func (s *Storage) Delete(rel string, recursive bool) error {
	abs, err := s.safeJoin(rel)
	if err != nil {
		return err
	}
	if recursive {
		return os.RemoveAll(abs)
	}
	return os.Remove(abs)
}

// ZipPath zips the provided absolute directory or file path into the provided writer.
func (s *Storage) ZipPath(abs string, zw *zip.Writer) error {
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	base := filepath.Base(abs)
	if !info.IsDir() {
		return addFileToZip(zw, abs, base)
	}
	return filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(abs, path)
		rel = filepath.ToSlash(filepath.Join(base, rel))
		return addFileToZip(zw, path, rel)
	})
}

func addFileToZip(zw *zip.Writer, path string, nameInZip string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, _ := f.Stat()
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = nameInZip
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// Helpers
func (s *Storage) safeJoin(rel string) (string, error) {
	if rel == "" {
		rel = "."
	}
	cleaned := filepath.Clean(rel)
	abs := filepath.Join(s.Root, cleaned)
	abs = filepath.Clean(abs)
	rootClean := filepath.Clean(s.Root)
	if !strings.HasPrefix(abs+string(os.PathSeparator), rootClean+string(os.PathSeparator)) && abs != rootClean {
		return "", ErrBadPath
	}
	return abs, nil
}

func extractZip(r io.ReaderAt, size int64, dest string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		fp := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fp, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fp)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

// acquire returns an unlock func for a per-repo/branch key.
func (s *Storage) acquire(user, repo, branch string) func() {
	key := fmt.Sprintf("%s|%s|%s", user, repo, branch)
	s.mu.Lock()
	if s.lock == nil {
		s.lock = make(map[string]*sync.Mutex)
	}
	m, ok := s.lock[key]
	if !ok {
		m = &sync.Mutex{}
		s.lock[key] = m
	}
	s.mu.Unlock()
	m.Lock()
	return m.Unlock
}

func (s *Storage) downloadAndExtract(ctx context.Context, ownerRepo, branch, token, dest string) error {
	url := fmt.Sprintf("https://codeload.github.com/%s/zip/%s", ownerRepo, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/zip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("download archive failed: %s", string(b))
	}
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return err
	}
	// Extract into dest
	if err := extractZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()), dest); err != nil {
		return err
	}
	return nil
}

// Touch records access time for a relative path (dir). It ignores missing paths.
func (s *Storage) Touch(rel string) error {
	abs, err := s.safeJoin(rel)
	if err != nil {
		return err
	}
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return s.touchDir(abs)
	}
	return nil
}

func (s *Storage) touchDir(abs string) error {
	marker := filepath.Join(abs, ".last_access")
	now := []byte(time.Now().UTC().Format(time.RFC3339Nano))
	return os.WriteFile(marker, now, 0o644)
}

// CleanupExpired removes repos unused beyond ttl under users/*/repos/*/*/<branch>.
func (s *Storage) CleanupExpired(ttl time.Duration) error {
	cutoff := time.Now().Add(-ttl)
	root := filepath.Join(s.Root, "users")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignore inaccessible
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(s.Root, path)
		parts := splitPath(rel)
		if len(parts) < 6 {
			return nil
		}
		// expect users/<user>/repos/<owner>/<repo>/<branch>
		if parts[0] != "users" || parts[2] != "repos" {
			return nil
		}
		// branch dir depth 6
		if len(parts) >= 6 {
			if expired(path, cutoff) {
				_ = os.RemoveAll(path)
				return filepath.SkipDir
			}
		}
		return nil
	})
}

func expired(path string, cutoff time.Time) bool {
	marker := filepath.Join(path, ".last_access")
	if info, err := os.Stat(marker); err == nil {
		return info.ModTime().Before(cutoff)
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.ModTime().Before(cutoff)
}

func splitPath(p string) []string {
	p = filepath.ToSlash(p)
	parts := strings.Split(p, "/")
	out := parts[:0]
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}
