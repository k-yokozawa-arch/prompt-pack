package auth

import (
"os"
"strconv"
"time"
)

// Config holds authentication-related configuration.
type Config struct {
// APIKeyHashAlgorithm specifies the hashing algorithm (bcrypt or argon2).
APIKeyHashAlgorithm string
// BcryptCost is the bcrypt cost factor (default: 12).
BcryptCost int
// Argon2Time is the argon2 time parameter.
Argon2Time uint32
// Argon2Memory is the argon2 memory parameter in KB.
Argon2Memory uint32
// Argon2Threads is the argon2 parallelism parameter.
Argon2Threads uint8
// KeyRotationWindow is the grace period for old keys during rotation.
KeyRotationWindow time.Duration
// RateLimitPerMinute is the auth rate limit per API key.
RateLimitPerMinute int
// KeyCacheTTL is how long to cache validated keys.
KeyCacheTTL time.Duration
// EnableAuditLog enables authentication audit logging.
EnableAuditLog bool
}

// LoadConfig loads auth configuration from environment variables.
func LoadConfig() Config {
return Config{
APIKeyHashAlgorithm: getenv("AUTH_HASH_ALGORITHM", "bcrypt"),
BcryptCost:          getInt("AUTH_BCRYPT_COST", 12),
Argon2Time:          uint32(getInt("AUTH_ARGON2_TIME", 1)),
Argon2Memory:        uint32(getInt("AUTH_ARGON2_MEMORY", 64*1024)),
Argon2Threads:       uint8(getInt("AUTH_ARGON2_THREADS", 4)),
KeyRotationWindow:   getDuration("AUTH_KEY_ROTATION_WINDOW", 24*time.Hour),
RateLimitPerMinute:  getInt("AUTH_RATE_PER_MIN", 100),
KeyCacheTTL:         getDuration("AUTH_KEY_CACHE_TTL", 5*time.Minute),
EnableAuditLog:      getBool("AUTH_ENABLE_AUDIT", true),
}
}

func getenv(key, def string) string {
if v, ok := os.LookupEnv(key); ok && v != "" {
return v
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

func getDuration(key string, def time.Duration) time.Duration {
if v, ok := os.LookupEnv(key); ok {
if d, err := time.ParseDuration(v); err == nil {
return d
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
