package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	Host       string
	Port       int
	DBPath     string
	Demo       bool
	NoBrowser  bool
	APIKey     string
	Model      string
	BaseURL    string
	ClientName string
	// InputRoot enables raw artifact references (file:<relative path>) under
	// this directory. Empty disables raw references.
	InputRoot string
	// HeavyDir enables the HEAVY profile with the local Heavy Execution
	// Adapter persisting job state in this directory. Empty disables HEAVY.
	HeavyDir      string
	AllowedModels []string
	// NoWeb serves only the API (/api, /api/public/v1) without the embedded
	// Reference Web.
	NoWeb bool
}

func ParseConfig(args []string) (*Config, error) {
	fs := flag.NewFlagSet("insight-lab", flag.ContinueOnError)
	build := BindFlags(fs)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return build()
}

// BindFlags registers the engine configuration flags on fs and returns a
// function that builds the Config after fs.Parse. Server and headless CLI
// commands share it, so every capability is configured the same way.
func BindFlags(fs *flag.FlagSet) func() (*Config, error) {
	host := fs.String("host", "127.0.0.1", "bind address")
	port := fs.Int("port", 8787, "HTTP port")
	dbPath := fs.String("db", "", "SQLite database path (default: OS data dir)")
	demo := fs.Bool("demo", false, "load the demo dataset and open the browser (demo builds only)")
	noBrowser := fs.Bool("no-browser", false, "do not open a browser automatically")
	noWeb := fs.Bool("no-web", false, "serve only the API; do not serve the embedded Reference Web")
	apiKey := fs.String("api-key", os.Getenv("INSIGHT_LAB_API_KEY"), "LLM API key")
	model := fs.String("model", "", "LLM model name")
	baseURL := fs.String("base-url", "", "OpenAI-compatible base URL")
	inputRoot := fs.String("input-root", os.Getenv("INSIGHT_LAB_INPUT_ROOT"), "directory that file: raw artifact references may read (empty disables raw references)")
	allowedModels := fs.String("allowed-models", os.Getenv("INSIGHT_LAB_ALLOWED_MODELS"), "comma-separated models (besides -model) callers may bind to pipeline stages via modelBindings")
	heavyDir := fs.String("heavy-dir", os.Getenv("INSIGHT_LAB_HEAVY_DIR"), "directory for local HEAVY job state (empty disables the HEAVY profile)")
	clientName := fs.String("client", os.Getenv("INSIGHT_LAB_CLIENT_NAME"), "client name shown in the delivery build's confidentiality banner")
	return func() (*Config, error) {
		path := *dbPath
		if path == "" {
			dir, err := defaultDataDir()
			if err != nil {
				return nil, err
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("create data dir: %w", err)
			}
			path = filepath.Join(dir, "insight.db")
		}
		return &Config{
			Host: *host, Port: *port, DBPath: path, Demo: *demo, NoBrowser: *noBrowser, NoWeb: *noWeb,
			APIKey: *apiKey, Model: *model, BaseURL: *baseURL, ClientName: *clientName,
			InputRoot: *inputRoot, HeavyDir: *heavyDir, AllowedModels: splitList(*allowedModels),
		}, nil
	}
}

func defaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "InsightLab"), nil
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "InsightLab"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Roaming", "InsightLab"), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "insight-lab"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "insight-lab"), nil
	}
}

func splitList(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
