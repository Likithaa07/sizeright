package api

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

type rateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	perMin   int
}

// RateLimit allows each user (or IP if not logged in) perMin requests per minute.
// Uses a "token bucket": the bucket refills steadily and allows short bursts.
func RateLimit(perMin int) func(http.Handler) http.Handler {
	rl := &rateLimiter{limiters: map[string]*rate.Limiter{}, perMin: perMin}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := userFrom(r).ID
			if key == "" {
				key, _, _ = net.SplitHostPort(r.RemoteAddr)
			}
			if !rl.get(key).Allow() {
				w.Header().Set("Retry-After", "60")
				writeError(w, r, http.StatusTooManyRequests, "rate limit exceeded, try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *rateLimiter) get(key string) *rate.Limiter {
	rl.mu.Lock() // many requests run at once, so protect the map
	defer rl.mu.Unlock()
	l, ok := rl.limiters[key]
	if !ok {
		l = rate.NewLimiter(rate.Limit(float64(rl.perMin)/60), rl.perMin)
		rl.limiters[key] = l
	}
	return l
}
