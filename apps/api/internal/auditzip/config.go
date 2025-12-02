package auditzip

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	S3Endpoint         string
	S3Bucket           string
	SignURLTTL         time.Duration
	RetentionPeriod    time.Duration
	MaxRangeDays       int
	EstimatedMBPerDay  float64
	SplitChunkMB       float64
	MaxQueueDepth      int
	MaxConcurrentJobs  int
	MaxRetries         int
	RetryBaseDelay     time.Duration
	RateLimitPerMinute int
	QueueRetryAfter    time.Duration
	DefaultLocale      string
	DefaultTimeZone    string
	EnableSSE          bool
	KMSKeyID           string
	AllowedOrigins     []string
}

func LoadConfig() Config {
	return Config{
		S3Endpoint:         getenv("S3_ENDPOINT", "https://s3.example.com"),
		S3Bucket:           getenv("AUDIT_S3_BUCKET", "audit-archives"),
		SignURLTTL:         getDuration("AUDIT_SIGN_URL_TTL", 10*time.Minute),
		RetentionPeriod:    time.Duration(getInt("AUDIT_RETENTION_DAYS", 7)) * 24 * time.Hour,
		MaxRangeDays:       getInt("AUDIT_MAX_RANGE_DAYS", 92),
		EstimatedMBPerDay:  getFloat("AUDIT_EST_MB_PER_DAY", 5.0),
		SplitChunkMB:       getFloat("AUDIT_SPLIT_CHUNK_MB", 100.0),
		MaxQueueDepth:      getInt("AUDIT_MAX_QUEUE_DEPTH", 100),
		MaxConcurrentJobs:  max(1, getInt("AUDIT_MAX_CONCURRENCY", 4)),
		MaxRetries:         max(1, getInt("AUDIT_MAX_RETRIES", 3)),
		RetryBaseDelay:     getDuration("AUDIT_RETRY_BASE_DELAY", 2*time.Second),
		RateLimitPerMinute: getInt("AUDIT_RATE_PER_MIN", 60),
		QueueRetryAfter:    getDuration("AUDIT_RETRY_AFTER", 30*time.Second),
		DefaultLocale:      getenv("DEFAULT_LOCALE", "ja-JP"),
		DefaultTimeZone:    getenv("DEFAULT_TZ", "Asia/Tokyo"),
		EnableSSE:          getBool("AUDIT_SSE_ENABLED", true),
		KMSKeyID:           getenv("AUDIT_KMS_KEY", ""),
		AllowedOrigins:     splitList(getenv("AUDIT_ALLOWED_ORIGINS", "http://localhost:3000")),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
	}
	return def
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
