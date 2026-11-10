package cache

import (
	"context"
	"testing"
	"time"
)

// Drill 2 regression test: with Redis down, cache calls must fail fast
// (a miss), not eat the request's time budget.
func TestRedisDownFailsFast(t *testing.T) {
	c := New("127.0.0.1:1") // nothing listens on port 1
	ctx := context.Background()

	start := time.Now()
	_, hit := c.Get(ctx, "k")
	c.Set(ctx, "k", []byte("v"))
	took := time.Since(start)

	if hit {
		t.Fatal("expected a miss when Redis is down")
	}
	if took > 300*time.Millisecond {
		t.Fatalf("cache calls took %v with Redis down; want < 300ms", took)
	}
}
