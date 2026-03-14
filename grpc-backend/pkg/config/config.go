package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL   string
	OpenSearchURL string
	EmbedModelID  string
	ServerAddr    string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env file is optional, env vars take precedence
	return &Config{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://playground:playground@localhost:5432/playground?sslmode=disable"),
		OpenSearchURL: getEnv("OPENSEARCH_URL", "http://localhost:9200"),
		EmbedModelID:  getEnv("EMBED_MODEL_ID", ""),
		ServerAddr:    getEnv("SERVER_ADDR", ":8080"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
