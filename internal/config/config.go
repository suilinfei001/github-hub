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
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
	User    string `json:"user"`
}

func Default() Config {
	return Config{
		BaseURL: "http://localhost:8080",
		User:    "",
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
	if strings.HasPrefix(trimmed, "base_url:") {
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
//	user: "..."
func parseYAMLConfig(s string) (Config, error) {
	cfg := Default()
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimRight(raw, "\r")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		// Key: value pairs
		kv := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		v = strings.Trim(v, "\"'")
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
