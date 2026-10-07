package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cashi/cashi/backend/internal/cashback"
	"github.com/cashi/cashi/backend/internal/config"
	"github.com/cashi/cashi/backend/internal/httpapi"
	"github.com/cashi/cashi/backend/internal/postgres"
	"github.com/cashi/cashi/backend/internal/redisstore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	rdb, err := redisstore.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer func() { _ = rdb.Close() }()

	svc, err := cashback.New(db, rdb)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(svc, db, rdb).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("API listening on %s (APP_ENV=%s)", cfg.Addr, cfg.Env)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
