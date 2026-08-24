// Package config loads server configuration from environment variables.
package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds everything the server needs to boot.
type Config struct {
	CatalogPath      string // path to catalog.yaml
	DataDir          string // directory containing the parquet files
	EmbeddingsAPIKey string // key for the external embeddings service (empty for local Ollama)
	EmbeddingsAPIURL string // base URL for the embeddings service
	EmbeddingsModel  string // model name to request from the embeddings service
}

// Load reads configuration from environment variables. Only path-ish values
// get local fallbacks; embeddings service settings depend on whatever the
// developer is running, so those come from .env with no hardcoded default.
func Load() Config {
	// Load .env if available
	_ = godotenv.Load()

	return Config{
		CatalogPath:      getEnv("CATALOG_PATH", "catalog/catalog.yaml"),
		DataDir:          getEnv("DATA_DIR", "data"),
		EmbeddingsAPIKey: os.Getenv("EMBEDDINGS_API_KEY"),
		EmbeddingsAPIURL: os.Getenv("EMBEDDINGS_API_URL"),
		EmbeddingsModel:  os.Getenv("EMBEDDINGS_MODEL"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
