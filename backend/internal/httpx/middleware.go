package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		identifier := c.GetHeader("X-Request-ID")
		if !requestIDPattern.MatchString(identifier) {
			bytes := make([]byte, 16)
			if _, err := rand.Read(bytes); err == nil {
				identifier = hex.EncodeToString(bytes)
			} else {
				identifier = uuid.NewString()
			}
		}
		c.Set("request_id", identifier)
		c.Header("X-Request-ID", identifier)
		c.Next()
	}
}

func SecurityHeaders(production bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		if production {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		_, ok := allowed[origin]
		if origin != "" && ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, Idempotency-Key")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			if origin != "" && !ok {
				Error(c, http.StatusForbidden, "origin_not_allowed", "The request origin is not allowed.")
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error("request panic", "request_id", c.GetString("request_id"))
		Error(c, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
	})
}

type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	lastSeen map[string]time.Time
	rate     rate.Limit
	burst    int
}

func NewIPRateLimiter(perMinute int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter), lastSeen: make(map[string]time.Time),
		rate: rate.Limit(float64(perMinute) / 60.0), burst: perMinute,
	}
}

func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		l.mu.Lock()
		limiter := l.limiters[ip]
		if limiter == nil {
			limiter = rate.NewLimiter(l.rate, l.burst)
			l.limiters[ip] = limiter
		}
		l.lastSeen[ip] = now
		if len(l.limiters) > 10000 {
			for key, seen := range l.lastSeen {
				if now.Sub(seen) > time.Hour {
					delete(l.lastSeen, key)
					delete(l.limiters, key)
				}
			}
		}
		allowed := limiter.Allow()
		l.mu.Unlock()
		if !allowed {
			c.Header("Retry-After", "60")
			Error(c, http.StatusTooManyRequests, "rate_limited", "Too many requests. Try again later.")
			return
		}
		c.Next()
	}
}

func Audit(db *gorm.DB, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		event := models.AuditEvent{
			ID: uuid.NewString(), UserID: c.GetString("user_id"), Action: action,
			Resource: c.FullPath(), RequestID: c.GetString("request_id"),
			Status: c.Writer.Status(), IPAddress: c.ClientIP(), CreatedAt: time.Now().UTC(),
		}
		if err := db.Create(&event).Error; err != nil {
			slog.Error("audit event write failed", "request_id", event.RequestID)
		}
	}
}
