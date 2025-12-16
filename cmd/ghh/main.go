package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	ic "github-hub/internal/client"
	cfgpkg "github-hub/internal/config"
	"github-hub/internal/version"
)

const defaultTimeout = 30 * time.Second

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	// Global flags
	server := getenvDefault("GHH_BASE_URL", "")
	token := os.Getenv("GHH_TOKEN")
	timeout := defaultTimeout
	insecure := false
	configPath := os.Getenv("GHH_CONFIG")
	user := strings.TrimSpace(os.Getenv("GHH_USER"))
	apiPrefix := ""
	apiDownload := ""
	apiBranchSwitch := ""
	apiDirList := ""
	apiDirDelete := ""
	showVersion := false

	global := flag.NewFlagSet("ghh", flag.ContinueOnError)
	global.StringVar(&server, "server", server, "server base URL (env: GHH_BASE_URL or config.base_url)")
	global.StringVar(&token, "token", token, "auth token (env: GHH_TOKEN)")
	global.StringVar(&user, "user", user, "user name (env: GHH_USER or config.user)")
	global.DurationVar(&timeout, "timeout", timeout, "HTTP timeout")
	global.BoolVar(&insecure, "insecure", insecure, "skip TLS verification")
	global.StringVar(&configPath, "config", configPath, "path to YAML config (env: GHH_CONFIG); JSON compatible")
	global.StringVar(&apiPrefix, "api-prefix", apiPrefix, "prefix to prepend to all API paths")
	global.StringVar(&apiDownload, "api-download", apiDownload, "download API path template")
	global.StringVar(&apiBranchSwitch, "api-branch-switch", apiBranchSwitch, "branch switch API path template")
	global.StringVar(&apiDirList, "api-dir-list", apiDirList, "dir list API path template")
	global.StringVar(&apiDirDelete, "api-dir-delete", apiDirDelete, "dir delete API path template")
	global.BoolVar(&showVersion, "version", showVersion, "print version and exit")

	// Parse global flags followed by subcommands.
	// Example: ghh --server http://... download --repo foo --branch main --dest out.zip
	// We parse only up to the first non-flag arg (the subcommand), then reparse per-subcommand.
	if err := global.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if showVersion {
		fmt.Println(version.String())
		return
	}

	args := global.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(2)
	}

	// Load config and merge with flags
	cfg, err := cfgpkg.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}
	if server == "" {
		server = cfg.BaseURL
	}
	if token == "" && cfg.Token != "" {
		token = cfg.Token
	}
	if strings.TrimSpace(user) == "" && strings.TrimSpace(cfg.User) != "" {
		user = cfg.User
	}
	eps := ic.DefaultEndpoints()
	// Per-flag overrides
	if apiDownload != "" {
		eps.Download = apiDownload
	}
	if apiBranchSwitch != "" {
		eps.BranchSwitch = apiBranchSwitch
	}
	if apiDirList != "" {
		eps.DirList = apiDirList
	}
	if apiDirDelete != "" {
		eps.DirDelete = apiDirDelete
	}
	if apiPrefix != "" {
		// Prepend prefix if paths are absolute (start with "/")
		prepend := func(p string) string {
			if strings.HasPrefix(p, "/") {
				return "/" + strings.Trim(apiPrefix, "/") + p
			}
			return p
		}
		eps.Download = prepend(eps.Download)
		eps.BranchSwitch = prepend(eps.BranchSwitch)
		eps.DirList = prepend(eps.DirList)
		eps.DirDelete = prepend(eps.DirDelete)
	}

	if server == "" {
		server = "http://localhost:8080"
	}

	// Build HTTP client
	transport := &http.Transport{}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 optional
	}
	httpClient := &http.Client{Timeout: timeout, Transport: transport}
	client := ic.NewClient(server, token, httpClient)
	client.Endpoint = eps
	client.User = strings.TrimSpace(user)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch args[0] {
	case "download":
		cmd := flag.NewFlagSet("download", flag.ExitOnError)
		repo := cmd.String("repo", "", "repository identifier (e.g. owner/name or name)")
		branch := cmd.String("branch", "", "branch name (default: server default)")
		dest := cmd.String("dest", "", "destination path (default: current directory)")
		extract := cmd.Bool("extract", false, "extract zip archive into dest directory")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		if *repo == "" {
			fmt.Fprintln(os.Stderr, "download requires --repo")
			os.Exit(2)
		}
		zipPath, extractDir := resolveDest(*repo, *dest, *extract)
		if err := client.Download(ctx, *repo, *branch, zipPath, extractDir); err != nil {
			exitErr(err)
		}

	case "switch":
		cmd := flag.NewFlagSet("switch", flag.ExitOnError)
		repo := cmd.String("repo", "", "repository identifier")
		branch := cmd.String("branch", "", "branch to switch to")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		if *repo == "" || *branch == "" {
			fmt.Fprintln(os.Stderr, "switch requires --repo and --branch")
			os.Exit(2)
		}
		if err := client.SwitchBranch(ctx, *repo, *branch); err != nil {
			exitErr(err)
		}

	case "ls":
		cmd := flag.NewFlagSet("ls", flag.ExitOnError)
		path := cmd.String("path", ".", "remote path to list")
		raw := cmd.Bool("raw", false, "print raw JSON returned by server")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		if err := client.ListDir(ctx, *path, *raw); err != nil {
			exitErr(err)
		}

	case "rm":
		cmd := flag.NewFlagSet("rm", flag.ExitOnError)
		path := cmd.String("path", "", "remote path to delete")
		recursive := cmd.Bool("r", false, "recursive delete")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		if *path == "" {
			fmt.Fprintln(os.Stderr, "rm requires --path")
			os.Exit(2)
		}
		if err := client.DeleteDir(ctx, *path, *recursive); err != nil {
			exitErr(err)
		}

	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		os.Exit(2)
	}
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func exitErr(err error) {
	if err == nil {
		return
	}
	var he *ic.HTTPError
	if errors.As(err, &he) {
		fmt.Fprintf(os.Stderr, "error: %s (status=%d)\n", he.Message, he.StatusCode)
		if he.Body != "" {
			fmt.Fprintln(os.Stderr, he.Body)
		}
	} else {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(1)
}

func printUsage() {
	fmt.Print(`ghh - GitHub Hub client (offline-friendly)

Usage:
  ghh [--server URL] [--token TOKEN] [--config PATH] <command> [flags]

Commands:
  download   Download repository code as archive (optionally extract)
  switch     Switch repository branch on server
  ls         List remote directory contents
  rm         Delete remote directory (use -r for recursive)
  help       Show this help message

Global Flags:
  --server     Server base URL (env: GHH_BASE_URL) (default: http://localhost:8080)
  --token      Auth token (env: GHH_TOKEN)
  --user       User name for grouping cache (env: GHH_USER)
  --config     Path to YAML config (env: GHH_CONFIG); JSON compatible
  --timeout    HTTP timeout (default: 30s)
  --insecure   Skip TLS verification
  --api-prefix Prefix to prepend to all API paths
  --api-download, --api-branch-switch, --api-dir-list, --api-dir-delete to override individual endpoints
  --version    Print version and exit

Examples:
  ghh --server http://localhost:8080 download --repo foo/bar --branch main
  ghh --server http://localhost:8080 download --repo foo/bar --dest out.zip
  ghh --server http://localhost:8080 download --repo foo --extract
  ghh --server http://localhost:8080 switch --repo foo/bar --branch dev
  ghh --server http://localhost:8080 ls --path /mirrors/foo
  ghh --server http://localhost:8080 rm --path /mirrors/foo --r
`)
}

// resolveDest determines the zip file path and extract directory based on repo and dest flag.
// Returns (zipPath, extractDir):
// - zipPath: where to save the zip file
// - extractDir: where to extract (empty if extract=false, or same as zip's parent dir)
func resolveDest(repo, dest string, extract bool) (zipPath, extractDir string) {
	// Extract repo name from owner/repo
	repoName := repo
	if idx := strings.LastIndex(repoName, "/"); idx >= 0 {
		repoName = repoName[idx+1:]
	}

	// Determine the base directory and zip file path
	if dest == "" {
		// Default to current directory
		zipPath = "./" + repoName + ".zip"
		if extract {
			extractDir = "."
		}
		return
	}

	// If dest is an existing directory, save zip inside it
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		zipPath = filepath.Join(dest, repoName+".zip")
		if extract {
			extractDir = dest
		}
		return
	}

	// dest is a file path (or non-existent path), use as-is
	zipPath = dest
	if extract {
		// Extract to the directory containing the zip file
		extractDir = filepath.Dir(dest)
		if extractDir == "" {
			extractDir = "."
		}
	}
	return
}
