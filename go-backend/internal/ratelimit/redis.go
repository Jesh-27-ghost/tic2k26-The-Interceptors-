package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// luaTokenBucket is an atomic token bucket script run on Redis.
var luaTokenBucket = redis.NewScript(`
local key_tokens = KEYS[1]
local key_ts     = KEYS[2]

local capacity    = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now         = tonumber(ARGV[3])
local requested   = tonumber(ARGV[4])

-- Get current state
local tokens  = tonumber(redis.call('GET', key_tokens) or capacity)
local last_ts = tonumber(redis.call('GET', key_ts) or now)

-- Refill tokens based on elapsed time
local elapsed = math.max(0, now - last_ts)
local new_tokens = math.min(capacity, tokens + (elapsed * refill_rate))

-- Try to consume
local allowed = 0
if new_tokens >= requested then
	new_tokens = new_tokens - requested
	allowed = 1
end

-- Persist state with TTL (auto-cleanup idle keys)
redis.call('SET', key_tokens, new_tokens, 'EX', 300)
redis.call('SET', key_ts, now, 'EX', 300)

return allowed
`)

// RateLimiter manages rate limiting using Redis token bucket algorithm.
type RateLimiter struct {
	client     *redis.Client
	capacity   int64
	refillRate float64 // tokens per second
}

// NewRateLimiter creates a new Redis-backed token bucket rate limiter.
// For 60 requests/minute, capacity=60, refillRate=1.0.
func NewRateLimiter(client *redis.Client, capacity int64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		client:     client,
		capacity:   capacity,
		refillRate: refillRate,
	}
}

// Allow checks if the given API key and IP are allowed to make a request.
// It uses an atomic Lua script to evaluate the token bucket.
func (rl *RateLimiter) Allow(ctx context.Context, apiKey string, ip string) (bool, error) {
	keyTokens := fmt.Sprintf("sp:rl:%s:%s:tokens", apiKey, ip)
	keyTs := fmt.Sprintf("sp:rl:%s:%s:ts", apiKey, ip)

	nowStr := fmt.Sprintf("%f", float64(time.Now().UnixNano())/1e9)

	result, err := luaTokenBucket.Run(ctx, rl.client,
		[]string{keyTokens, keyTs},           // KEYS
		rl.capacity, rl.refillRate, nowStr, 1, // ARGV
	).Result()

	if err != nil {
		return false, fmt.Errorf("redis token bucket error: %w", err)
	}

	allowed, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis script result type")
	}

	return allowed == 1, nil
}
