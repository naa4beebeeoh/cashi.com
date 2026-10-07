package redisstore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
}

func Connect(ctx context.Context, redisURL string) (*Store, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Store{client: client}, nil
}

func (s *Store) Close() error {
	return s.client.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// TryLock acquires a short-lived lock for an idempotency key.
// Returns false if another request holds the lock (in-flight duplicate).
func (s *Store) TryLock(ctx context.Context, scope, userID, key string, ttl time.Duration) (bool, error) {
	ok, err := s.client.SetNX(ctx, lockKey(scope, userID, key), "1", ttl).Result()
	return ok, err
}

func (s *Store) Unlock(ctx context.Context, scope, userID, key string) error {
	return s.client.Del(ctx, lockKey(scope, userID, key)).Err()
}

func lockKey(scope, userID, key string) string {
	return fmt.Sprintf("idempotency:lock:%s:%s:%s", scope, userID, key)
}
