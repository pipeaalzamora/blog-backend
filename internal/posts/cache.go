package posts

import (
	"fmt"
	"sync"
	"time"
)

type cachedPosts struct {
	posts     []Post
	total     int64
	expiresAt time.Time
}

var (
	mu    sync.RWMutex
	cache = map[string]*cachedPosts{}
)

func cacheKey(page, limit int) string {
	return fmt.Sprintf("%d:%d", page, limit)
}

func getCache(page, limit int) ([]Post, int64, bool) {
	mu.RLock()
	defer mu.RUnlock()
	c, ok := cache[cacheKey(page, limit)]
	if !ok || time.Now().After(c.expiresAt) {
		return nil, 0, false
	}
	return c.posts, c.total, true
}

func setCache(page, limit int, posts []Post, total int64) {
	mu.Lock()
	defer mu.Unlock()
	cache[cacheKey(page, limit)] = &cachedPosts{
		posts:     posts,
		total:     total,
		expiresAt: time.Now().Add(30 * time.Second),
	}
}

func InvalidateCache() {
	mu.Lock()
	defer mu.Unlock()
	cache = map[string]*cachedPosts{}
}
