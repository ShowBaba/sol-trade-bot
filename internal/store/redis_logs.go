package store

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	"github.com/ShowBaba/tradebot/internal/logbus"
)

const logsKey = "tradebot:logs"

// RedisLogStore appends every log event to Redis (RPUSH) and never deletes. Full history is kept.
type RedisLogStore struct {
	client *redis.Client
	key    string
}

// NewRedisLogStore returns a log store using the given Redis client (same DB as state). Key: tradebot:logs.
func NewRedisLogStore(client *redis.Client) *RedisLogStore {
	return &RedisLogStore{client: client, key: logsKey}
}

// Append persists one log event to Redis. Implements logbus.Persister (non-blocking). Safe to call from logbus.Publish.
func (r *RedisLogStore) Append(e logbus.Event) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisDefaultTimeout)
	defer cancel()
	_ = r.client.RPush(ctx, r.key, data).Err()
}

// GetHistory returns the most recent log events, oldest first. limit 0 or negative means return all (can be large).
func (r *RedisLogStore) GetHistory(limit int) ([]logbus.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisDefaultTimeout)
	defer cancel()
	var start, stop int64
	if limit > 0 {
		// LLEN then LRANGE -limit -1 so we get last 'limit' entries, oldest first
		n, err := r.client.LLen(ctx, r.key).Result()
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, nil
		}
		if int64(limit) >= n {
			start, stop = 0, -1
		} else {
			start = n - int64(limit)
			stop = -1
		}
	} else {
		start, stop = 0, -1
	}
	slice, err := r.client.LRange(ctx, r.key, start, stop).Result()
	if err != nil {
		return nil, err
	}
	out := make([]logbus.Event, 0, len(slice))
	for _, s := range slice {
		var e logbus.Event
		if err := json.Unmarshal([]byte(s), &e); err != nil {
			continue
		}
		// ensure Time is parsed if came as string
		if e.Time.IsZero() && s != "" {
			// already zero, skip or leave
		}
		out = append(out, e)
	}
	return out, nil
}
