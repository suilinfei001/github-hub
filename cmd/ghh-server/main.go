package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	srv "github-hub/internal/server"
)

func main() {
	configPath := findConfigPath(os.Args[1:])
	cfg, err := srv.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Env overrides config defaults for compatibility.
	if envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); envToken != "" {
		cfg.Token = envToken
	}

	addr := cfg.Addr
	root := cfg.Root
	token := cfg.Token
	defaultUser := cfg.DefaultUser

	flag.StringVar(&configPath, "config", configPath, "path to server config (yaml or json)")
	flag.StringVar(&addr, "addr", addr, "listen address (e.g., :8080)")
	flag.StringVar(&root, "root", root, "workspace root to store caches")
	flag.StringVar(&token, "github-token", token, "GitHub token for higher rate limits (env: GITHUB_TOKEN)")
	flag.StringVar(&defaultUser, "default-user", defaultUser, "default user grouping when client user is empty")
	flag.Parse()

	s, err := srv.NewServer(root, defaultUser, token)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:              addr,
		Handler:           logging(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Printf("ghh-server listening on %s, root=%s, default_user=%s\n", addr, root, defaultUser)
	log.Fatal(srv.ListenAndServe())
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s %s\n", r.Method, r.URL.Path, time.Since(start))
	})
}

// findConfigPath scans args for --config or -config to allow loading defaults before flag parsing.
func findConfigPath(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--config="))
		}
		if arg == "--config" || arg == "-config" {
			if i+1 < len(args) {
				return strings.TrimSpace(args[i+1])
			}
		}
	}
	return ""
}
