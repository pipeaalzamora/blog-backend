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

// gc elimina entradas de IPs que no han tenido actividad dentro de la ventana dada.
// Debe llamarse con el mutex ya tomado.
func (r *rateLimiter) gc(now time.Time, window time.Duration) {
	if now.Sub(r.lastGC) < gcInterval {
		return
	}
	for ip, times := range r.requests {
		hasRecent := false
		for _, t := range times {
			if now.Sub(t) < window {
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

// limit aplica un límite de max peticiones por IP dentro de la ventana indicada.
func (r *rateLimiter) limit(max int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		r.mu.Lock()

		// Limpiar IPs inactivas periódicamente para evitar memory leak
		r.gc(now, window)

		times := r.requests[ip]
		var recent []time.Time
		for _, t := range times {
			if now.Sub(t) < window {
				recent = append(recent, t)
			}
		}
		if len(recent) >= max {
			r.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		r.requests[ip] = append(recent, now)
		r.mu.Unlock()
		c.Next()
	}
}

// RateLimit limita las peticiones globales por IP a maxPerSecond por segundo.
func RateLimit(maxPerSecond int) gin.HandlerFunc {
	return limiter.limit(maxPerSecond, time.Second)
}
