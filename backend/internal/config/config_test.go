package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStrictEnvsRequireSecrets(t *testing.T) {
	for _, env := range []string{"staging", "production"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("REDIS_URL")

			_, err := Load()
			if err == nil {
				t.Fatal("expected error when secrets missing")
			}
		})
	}
}

func TestLoadProductionWithSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://prod/db")
	t.Setenv("REDIS_URL", "redis://prod/0")
	t.Setenv("API_ADDR", ":9090")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "production" || cfg.DatabaseURL != "postgres://prod/db" || cfg.Addr != ":9090" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestLoadDemoAllowsLocalDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "demo")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("REDIS_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "demo" {
		t.Fatalf("env=%q", cfg.Env)
	}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" {
		t.Fatal("expected local defaults")
	}
}

func TestLoadDotEnvDoesNotOverrideProcessEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("API_ADDR=:9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("API_ADDR", ":8080")
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("API_ADDR"); got != ":8080" {
		t.Fatalf("got %q", got)
	}
}
