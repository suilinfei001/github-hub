package version

import "testing"

func TestString(t *testing.T) {
	origV, origC, origD := Version, Commit, BuildDate
	defer func() {
		Version, Commit, BuildDate = origV, origC, origD
	}()

	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{
			name:    "default main",
			version: "main",
			commit:  "",
			date:    "",
			want:    "main",
		},
		{
			name:    "empty version falls back to dev",
			version: "",
			commit:  "",
			date:    "",
			want:    "dev",
		},
		{
			name:    "with commit",
			version: "v1.2.3",
			commit:  "abc123",
			date:    "",
			want:    "v1.2.3 (commit=abc123)",
		},
		{
			name:    "with date",
			version: "v1.2.3",
			commit:  "",
			date:    "2024-01-01T00:00:00Z",
			want:    "v1.2.3 (date=2024-01-01T00:00:00Z)",
		},
		{
			name:    "with commit and date",
			version: "v1.2.3",
			commit:  "abc123",
			date:    "2024-01-01T00:00:00Z",
			want:    "v1.2.3 (commit=abc123, date=2024-01-01T00:00:00Z)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version, Commit, BuildDate = tt.version, tt.commit, tt.date
			if got := String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
