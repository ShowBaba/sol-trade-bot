package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ShowBaba/tradebot/internal/agent"
)

const (
	keyPrefixAgents        = "tradebot:agents"
	keyPrefixDeletedAgents = "tradebot:deleted_agents"
	keyPrefixTrades        = "tradebot:trades"
	redisDefaultTimeout    = 5 * time.Second
)

// RedisBackend persists state in three keys: tradebot:agents, tradebot:deleted_agents, tradebot:trades. Nothing is deleted.
type RedisBackend struct {
	client *redis.Client
}

// RedisOptions configures the Redis connection. DB is the database index (0-15); if set to -1, the DB from the URL is used (or 0).
type RedisOptions struct {
	URL string // e.g. "redis://localhost:6379/0" or "localhost:6379"
	DB  int    // database index 0-15; -1 = use URL or default 0
}

// NewRedisClient returns a Redis client for the given URL and DB. Use this when you need one client for both state and logs.
func NewRedisClient(addr string, db int) (*redis.Client, error) {
	if addr == "" {
		addr = "localhost:6379"
	}
	opts, err := redis.ParseURL(addr)
	if err != nil {
		opts = &redis.Options{Addr: addr}
	}
	if db >= 0 {
		opts.DB = db
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), redisDefaultTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

// NewRedisBackend returns a StoreBackend that uses Redis. addr is the Redis URL; db is the database index (0-15). If db is negative, the DB from the URL is used (or 0).
func NewRedisBackend(addr string, db int) (*RedisBackend, error) {
	return NewRedisBackendWithOptions(RedisOptions{URL: addr, DB: db})
}

// NewRedisBackendWithOptions returns a StoreBackend using the given options.
func NewRedisBackendWithOptions(opts RedisOptions) (*RedisBackend, error) {
	client, err := NewRedisClient(opts.URL, opts.DB)
	if err != nil {
		return nil, err
	}
	return &RedisBackend{client: client}, nil
}

// NewRedisBackendWithClient returns a RedisBackend using an existing client (e.g. for testing).
func NewRedisBackendWithClient(client *redis.Client) *RedisBackend {
	return &RedisBackend{client: client}
}

// Load implements agent.StoreBackend.
func (r *RedisBackend) Load() (*agent.Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisDefaultTimeout)
	defer cancel()
	s := &agent.Store{}

	data, err := r.client.Get(ctx, keyPrefixAgents).Bytes()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &s.Agents)
	}

	data, err = r.client.Get(ctx, keyPrefixDeletedAgents).Bytes()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &s.DeletedAgents)
	}

	data, err = r.client.Get(ctx, keyPrefixTrades).Bytes()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &s.Trades)
	}

	return s, nil
}

// Save implements agent.StoreBackend.
func (r *RedisBackend) Save(s *agent.Store) error {
	if s == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisDefaultTimeout)
	defer cancel()
	pipe := r.client.Pipeline()
	if data, err := json.Marshal(s.Agents); err == nil {
		pipe.Set(ctx, keyPrefixAgents, data, 0)
	}
	if data, err := json.Marshal(s.DeletedAgents); err == nil {
		pipe.Set(ctx, keyPrefixDeletedAgents, data, 0)
	}
	if data, err := json.Marshal(s.Trades); err == nil {
		pipe.Set(ctx, keyPrefixTrades, data, 0)
	}
	_, err := pipe.Exec(ctx)
	return err
}
