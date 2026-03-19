// Package ratelimit implements a fixed-window rate limiter backed by Redis.
//
// Each API key gets its own counter per 60-second window. A Lua script makes
// the increment + TTL-set atomic so there are no race conditions at the edges
// of windows, even under high concurrency.
//
// Limits:
//
//	test keys — 100 requests / minute
//	live keys — 1000 requests / minute
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	LimitTest = 100  // requests per minute for test-env keys
	LimitLive = 1000 // requests per minute for live-env keys
	windowSec = 60
)

// Result is returned by Allow and contains everything needed to set HTTP headers.
type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time // when the current window expires
}

// Limiter is a Redis-backed fixed-window rate limiter.
type Limiter struct {
	rdb    *redis.Client
	script *redis.Script
}

// luaScript atomically increments a counter and sets its TTL on first write.
// Returns {count, ttl_remaining_seconds}.
var luaScript = redis.NewScript(`
local key     = KEYS[1]
local limit   = tonumber(ARGV[1])
local window  = tonumber(ARGV[2])

local count = redis.call('INCR', key)
if count == 1 then
    redis.call('EXPIRE', key, window)
end

local ttl = redis.call('TTL', key)
return {count, ttl}
`)

// New creates a Limiter connected to Redis at the given URL.
func New(redisURL string) (*Limiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	rdb := redis.NewClient(opts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Limiter{rdb: rdb, script: luaScript}, nil
}

// Close shuts down the Redis connection.
func (l *Limiter) Close() error {
	return l.rdb.Close()
}

// Allow checks whether keyID is within its rate limit for the given environment.
// It always increments the counter — call it once per request.
func (l *Limiter) Allow(ctx context.Context, keyID uuid.UUID, env string) Result {
	limit := LimitLive
	if env == "test" {
		limit = LimitTest
	}

	// Key: rl:<keyID>:<minute-bucket>  — one bucket per 60-second window
	bucket := time.Now().Unix() / windowSec
	key := fmt.Sprintf("rl:%s:%d", keyID.String(), bucket)

	vals, err := l.script.Run(ctx, l.rdb, []string{key}, limit, windowSec).Int64Slice()
	if err != nil {
		// Redis unavailable — fail open (don't block the request)
		return Result{Allowed: true, Limit: limit, Remaining: limit}
	}

	count := int(vals[0])
	ttl := int(vals[1])
	if ttl < 0 {
		ttl = windowSec
	}

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	return Result{
		Allowed:   count <= limit,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   time.Now().Add(time.Duration(ttl) * time.Second),
	}
}
