package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds client configuration loaded from YAML (preferred) or JSON (fallback).
type Config struct {
	BaseURL   string    `json:"base_url"`
	Token     string    `json:"token"`
	User      string    `json:"user"`
	Endpoints Endpoints `json:"endpoints"`
}

// Endpoints defines API paths; placeholders like {repo}, {branch}, {path} are supported.
type Endpoints struct {
	Download     string `json:"download"`      // e.g., "/api/v1/repos/{repo}/archive"
	BranchSwitch string `json:"branch_switch"` // e.g., "/api/v1/repos/{repo}/branch/{branch}"
	DirList      string `json:"dir_list"`      // e.g., "/api/v1/dir/list" or "/api/v1/dir/{path}"
	DirDelete    string `json:"dir_delete"`    // e.g., "/api/v1/dir" or "/api/v1/dir/{path}"
}

func Default() Config {
	return Config{
		BaseURL: "http://localhost:8080",
		User:    "",
		Endpoints: Endpoints{
			Download:     "/api/v1/download",
			BranchSwitch: "/api/v1/branch/switch",
			DirList:      "/api/v1/dir/list",
			DirDelete:    "/api/v1/dir",
		},
	}
}

// Load loads config from YAML (.yml/.yaml) or JSON path. If the file
// does not exist or path is empty, returns default config and no error.
func Load(path string) (Config, error) {
	if path == "" {
		return Default(), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}

	// Try YAML first (preferred)
	if isYAML(path) {
		if cfg, err := parseYAMLConfig(string(b)); err == nil {
			return cfg, nil
		} else {
			return Config{}, fmt.Errorf("yaml parse error: %w", err)
		}
	}

	// Fallback: detect by content if it looks like YAML
	trimmed := strings.TrimSpace(string(b))
	if strings.HasPrefix(trimmed, "base_url:") || strings.Contains(trimmed, "endpoints:") {
		cfg, err := parseYAMLConfig(string(b))
		if err != nil {
			return Config{}, fmt.Errorf("yaml parse error: %w", err)
		}
		return cfg, nil
	}

	// Fallback to JSON for compatibility
	cfg := Default()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func isYAML(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// Minimal YAML parser for the limited schema of Config.
// Supports:
//
//	base_url: "..."
//	token: "..."
//	endpoints:
//	  download: "/..."
//	  branch_switch: "/..."
//	  dir_list: "/..."
//	  dir_delete: "/..."
func parseYAMLConfig(s string) (Config, error) {
	cfg := Default()
	inEndpoints := false
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimRight(raw, "\r")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		// Section header
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(t, ":") {
			key := strings.TrimSuffix(t, ":")
			if key == "endpoints" {
				inEndpoints = true
				continue
			}
			inEndpoints = false
			continue
		}
		// Key: value pairs
		// Determine if this is an endpoints subkey by indentation
		indent := leadingSpaces(line)
		kv := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		v = strings.Trim(v, "\"'")
		if indent == 0 {
			inEndpoints = false
		}
		if inEndpoints && indent > 0 {
			switch k {
			case "download":
				if v != "" {
					cfg.Endpoints.Download = v
				}
			case "branch_switch":
				if v != "" {
					cfg.Endpoints.BranchSwitch = v
				}
			case "dir_list":
				if v != "" {
					cfg.Endpoints.DirList = v
				}
			case "dir_delete":
				if v != "" {
					cfg.Endpoints.DirDelete = v
				}
			}
			continue
		}
		// root level
		switch k {
		case "base_url":
			if v != "" {
				cfg.BaseURL = v
			}
		case "token":
			if v != "" {
				cfg.Token = v
			}
		case "user":
			if v != "" {
				cfg.User = v
			}
		}
	}
	return cfg, nil
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}
