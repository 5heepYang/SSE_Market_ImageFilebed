package core

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"imagebed/middleware"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type rateBucket struct {
	limiter *rate.Limiter
	last    time.Time
}

// keyedRateLimiter 按用户/客户端 IP 做 token bucket 限流。
// 未登录请求按 IP 限流；登录后按 user_id 限流，避免多用户共享 IP 时互相影响。
type keyedRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateBucket
	limit   rate.Limit
	burst   int
	ttl     time.Duration
}

func newKeyedRateLimiter(perSecond float64, burst int) *keyedRateLimiter {
	return &keyedRateLimiter{
		entries: make(map[string]*rateBucket),
		limit:   rate.Limit(perSecond),
		burst:   burst,
		ttl:     10 * time.Minute,
	}
}

func (l *keyedRateLimiter) allow(key string) bool {
	l.mu.Lock()
	now := time.Now()
	entry, ok := l.entries[key]
	if !ok {
		entry = &rateBucket{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = entry
	}
	entry.last = now
	l.mu.Unlock()
	return entry.limiter.Allow()
}

func (l *keyedRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for key, entry := range l.entries {
			if now.Sub(entry.last) > l.ttl {
				delete(l.entries, key)
			}
		}
		l.mu.Unlock()
	}
}

func rateLimitMiddleware(l *keyedRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "ip:" + c.ClientIP()
		if uid := middleware.UserID(c); uid != 0 {
			key = "user:" + strconv.FormatUint(uint64(uid), 10)
		}
		if !l.allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}
		c.Next()
	}
}

// tusRateLimitMiddleware tus 路由专用限流，区分两类请求：
//   - POST（新建上传会话）：走 admission 准入令牌桶（容量小、补充慢），控制新任务入场速率；
//   - 其余（PATCH 分片 / HEAD / DELETE / OPTIONS）：走 general 保护性限流（宽松值），
//     分片吞吐不能被准入速率卡住——真正的并发总量由 tusPreCreate 的在传上限兜底。
func tusRateLimitMiddleware(admission, general *keyedRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "ip:" + c.ClientIP()
		if uid := middleware.UserID(c); uid != 0 {
			key = "user:" + strconv.FormatUint(uint64(uid), 10)
		}
		limiter, message := general, "请求过于频繁，请稍后再试"
		if c.Request.Method == http.MethodPost {
			limiter, message = admission, "开始的上传任务过多，请稍后再试"
		}
		if !limiter.allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": message,
			})
			return
		}
		c.Next()
	}
}
