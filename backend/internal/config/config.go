package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env         string
	Addr        string
	DatabaseURL string
	RedisURL    string
}

func Load() (Config, error) {
	// Optional local file; deploy pipelines normally inject real env vars instead.
	_ = loadDotEnv(".env")

	env := strings.ToLower(strings.TrimSpace(getenv("APP_ENV", "local")))
	if env == "" {
		env = "local"
	}

	cfg := Config{
		Env:  env,
		Addr: getenv("API_ADDR", ":8080"),
	}

	strict := env == "staging" || env == "production"
	if strict {
		cfg.DatabaseURL = os.Getenv("DATABASE_URL")
		cfg.RedisURL = os.Getenv("REDIS_URL")
		if cfg.DatabaseURL == "" {
			return Config{}, fmt.Errorf("DATABASE_URL is required when APP_ENV=%s", env)
		}
		if cfg.RedisURL == "" {
			return Config{}, fmt.Errorf("REDIS_URL is required when APP_ENV=%s", env)
		}
	} else {
		cfg.DatabaseURL = getenv("DATABASE_URL", "postgres://cashi:cashi@localhost:5432/cashi?sslmode=disable")
		cfg.RedisURL = getenv("REDIS_URL", "redis://localhost:6379/0")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv sets env vars from a KEY=VALUE file without overriding existing process env.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
	return scanner.Err()
}
