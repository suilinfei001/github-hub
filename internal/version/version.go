package version

import "strings"

var (
	// Version is the semantic version of the binary. Overwrite at build time with
	// -ldflags "-X github-hub/internal/version.Version=vX.Y.Z".
	Version = "main"
	// Commit optionally carries a short git commit hash.
	Commit = ""
	// BuildDate optionally carries a UTC build timestamp, e.g. 2024-12-31T23:59:59Z.
	BuildDate = ""
)

// String returns a human-readable version string with optional metadata.
func String() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "dev"
	}
	var meta []string
	if c := strings.TrimSpace(Commit); c != "" {
		meta = append(meta, "commit="+c)
	}
	if d := strings.TrimSpace(BuildDate); d != "" {
		meta = append(meta, "date="+d)
	}
	if len(meta) == 0 {
		return v
	}
	return v + " (" + strings.Join(meta, ", ") + ")"
}
