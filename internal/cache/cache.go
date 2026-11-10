// Package cache stores sizing results in Redis so identical requests are reused.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func New(addr string) *Cache {
	return &Cache{
		rdb: redis.NewClient(&redis.Options{
			Addr:         addr,
			DialTimeout:  500 * time.Millisecond, // the cache is optional: never wait long for it
			ReadTimeout:  200 * time.Millisecond,
			WriteTimeout: 200 * time.Millisecond,
		}),
		ttl: 24 * time.Hour,
	}
}

// Key builds a cache key from the rules version and a hash of the input.
// Same input + same rules = same answer, so this is safe to cache.
func Key(rulesVersion string, input any) string {
	b, _ := json.Marshal(input)
	sum := sha256.Sum256(b)
	return "sizing:" + rulesVersion + ":" + hex.EncodeToString(sum[:])
}

// Get returns the cached value. Any error counts as a miss: if Redis is down,
// the app still works, just without the cache.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	return b, true
}

func (c *Cache) Set(ctx context.Context, key string, val []byte) {
	if c == nil {
		return
	}
	_ = c.rdb.Set(ctx, key, val, c.ttl).Err()
}
