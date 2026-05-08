package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	lastGC   time.Time
}

var limiter = &rateLimiter{
	requests: make(map[string][]time.Time),
	lastGC:   time.Now(),
}

// gcInterval define cada cuánto se limpian IPs inactivas del map.
const gcInterval = 5 * time.Minute

// gc elimina entradas de IPs que no han tenido actividad en el último segundo.
// Debe llamarse con el mutex ya tomado.
func (r *rateLimiter) gc(now time.Time) {
	if now.Sub(r.lastGC) < gcInterval {
		return
	}
	for ip, times := range r.requests {
		hasRecent := false
		for _, t := range times {
			if now.Sub(t) < time.Second {
				hasRecent = true
				break
			}
		}
		if !hasRecent {
			delete(r.requests, ip)
		}
	}
	r.lastGC = now
}

func RateLimit(maxPerSecond int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		limiter.mu.Lock()

		// Limpiar IPs inactivas periódicamente para evitar memory leak
		limiter.gc(now)

		times := limiter.requests[ip]
		var recent []time.Time
		for _, t := range times {
			if now.Sub(t) < time.Second {
				recent = append(recent, t)
			}
		}
		if len(recent) >= maxPerSecond {
			limiter.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		limiter.requests[ip] = append(recent, now)
		limiter.mu.Unlock()
		c.Next()
	}
}
