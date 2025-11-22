package auditzip

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	S3Endpoint      string
	S3Bucket        string
	SignURLTTL      time.Duration
	JobMaxDuration  time.Duration
	DefaultLocale   string
	DefaultTimeZone string
}

func LoadConfig() Config {
	return Config{
		S3Endpoint:      getenv("S3_ENDPOINT", "https://s3.example.com"),
		S3Bucket:        getenv("AUDIT_S3_BUCKET", "audit-archives"),
		SignURLTTL:      getDuration("SIGN_URL_TTL", 10*time.Minute),
		JobMaxDuration:  getDuration("JOB_MAX_DURATION", time.Minute),
		DefaultLocale:   getenv("DEFAULT_LOCALE", "ja-JP"),
		DefaultTimeZone: getenv("DEFAULT_TZ", "Asia/Tokyo"),
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
