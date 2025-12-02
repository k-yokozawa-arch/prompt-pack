package pint

import (
	"os"
	"strconv"
	"time"
)

// Config holds environment-driven settings for storage, validation, and signing.
type Config struct {
	S3Endpoint       string
	S3Bucket         string
	SignURLTTL       time.Duration
	MaxLines         int
	AllowedDelta     float64
	RoundingMode     string
	MaxDescription   int
	PDFEnabled       bool
	DefaultTimeZone  string
	DefaultLocale    string
	MaxParallelJobs  int
	EnableAuditHash  bool
	ValidUnitCodes   []string
	ValidTaxCategory []string
	PDFChromiumPath  string
	PDFTimeout       time.Duration
	PDFTmpDir        string
	PDFLocale        string
	PDFTimeZone      string
	PDFFontsDir      string
}

func LoadConfig() Config {
	return Config{
		S3Endpoint:       getenv("S3_ENDPOINT", "https://s3.example.com"),
		S3Bucket:         getenv("S3_BUCKET", "jp-pint-invoices"),
		SignURLTTL:       getDuration("SIGN_URL_TTL", 10*time.Minute),
		MaxLines:         getInt("MAX_INVOICE_LINES", 500),
		AllowedDelta:     getFloat("ALLOWED_TOTAL_DELTA", 0.01),
		RoundingMode:     getenv("ROUNDING_MODE", "HALF_UP"),
		MaxDescription:   getInt("MAX_DESCRIPTION_LEN", 240),
		PDFEnabled:       getBool("PDF_ENABLED", true),
		DefaultTimeZone:  getenv("DEFAULT_TZ", "Asia/Tokyo"),
		DefaultLocale:    getenv("DEFAULT_LOCALE", "ja-JP"),
		MaxParallelJobs:  getInt("MAX_PARALLEL_JOBS", 4),
		EnableAuditHash:  getBool("ENABLE_AUDIT_HASH", true),
		ValidUnitCodes:   []string{"EA", "HUR", "MTR", "D64", "KGM", "LTR"},
		ValidTaxCategory: []string{"S", "Z", "E", "O", "AE", "K", "G"},
		PDFChromiumPath:  getenv("PDF_CHROMIUM_PATH", ""),
		PDFTimeout:       getDuration("PDF_TIMEOUT", 15*time.Second),
		PDFTmpDir:        getenv("PDF_TMP_DIR", "/tmp"),
		PDFLocale:        getenv("PDF_LOCALE", "ja-JP"),
		PDFTimeZone:      getenv("PDF_TIMEZONE", "Asia/Tokyo"),
		PDFFontsDir:      getenv("PDF_FONTS_DIR", ""),
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
		if b, err := strconv.ParseBool(v); err == nil {
			return b
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
