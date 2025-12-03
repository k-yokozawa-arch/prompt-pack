package auth

import (
"crypto/rand"
"crypto/sha256"
"crypto/subtle"
"encoding/base64"
"encoding/hex"
"errors"
"fmt"
"strings"

"golang.org/x/crypto/argon2"
"golang.org/x/crypto/bcrypt"
)

// HashAlgorithm represents supported hashing algorithms.
type HashAlgorithm string

const (
AlgorithmBcrypt HashAlgorithm = "bcrypt"
AlgorithmArgon2 HashAlgorithm = "argon2"
)

// ErrInvalidKey indicates the key format is invalid.
var ErrInvalidKey = errors.New("invalid API key format")

// KeyPrefix is prepended to all API keys for easy identification.
const KeyPrefix = "ppk_" // prompt-pack key

// GenerateAPIKey generates a new API key with the format: ppk_<random>
// Returns the raw key (to show user once) and the prefix (for identification).
func GenerateAPIKey() (rawKey, prefix string, err error) {
// Generate 32 bytes of random data
keyBytes := make([]byte, 32)
if _, err := rand.Read(keyBytes); err != nil {
return "", "", fmt.Errorf("failed to generate random key: %w", err)
}

// Encode as base64url (URL-safe, no padding)
encoded := base64.RawURLEncoding.EncodeToString(keyBytes)
rawKey = KeyPrefix + encoded

// Prefix is first 8 characters after ppk_
if len(encoded) >= 8 {
prefix = encoded[:8]
} else {
prefix = encoded
}

return rawKey, prefix, nil
}

// HashKey hashes an API key using the specified algorithm.
func HashKey(rawKey string, cfg Config) (string, error) {
// Remove prefix if present
keyData := strings.TrimPrefix(rawKey, KeyPrefix)
if keyData == rawKey {
// No prefix found - invalid format
return "", ErrInvalidKey
}

switch HashAlgorithm(cfg.APIKeyHashAlgorithm) {
case AlgorithmBcrypt:
return hashBcrypt(keyData, cfg.BcryptCost)
case AlgorithmArgon2:
return hashArgon2(keyData, cfg)
default:
return hashBcrypt(keyData, cfg.BcryptCost)
}
}

// VerifyKey verifies a raw key against a stored hash.
func VerifyKey(rawKey, storedHash string, cfg Config) bool {
keyData := strings.TrimPrefix(rawKey, KeyPrefix)
if keyData == rawKey {
return false
}

// Detect algorithm from hash prefix
if strings.HasPrefix(storedHash, "$2") {
return verifyBcrypt(keyData, storedHash)
}
if strings.HasPrefix(storedHash, "$argon2") {
return verifyArgon2(keyData, storedHash, cfg)
}

// Unknown format
return false
}

// hashBcrypt hashes using bcrypt.
func hashBcrypt(data string, cost int) (string, error) {
hash, err := bcrypt.GenerateFromPassword([]byte(data), cost)
if err != nil {
return "", fmt.Errorf("bcrypt hash failed: %w", err)
}
return string(hash), nil
}

// verifyBcrypt verifies a bcrypt hash.
func verifyBcrypt(data, hash string) bool {
err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(data))
return err == nil
}

// hashArgon2 hashes using Argon2id.
func hashArgon2(data string, cfg Config) (string, error) {
// Generate salt
salt := make([]byte, 16)
if _, err := rand.Read(salt); err != nil {
return "", fmt.Errorf("failed to generate salt: %w", err)
}

hash := argon2.IDKey(
[]byte(data),
salt,
cfg.Argon2Time,
cfg.Argon2Memory,
cfg.Argon2Threads,
32,
)

// Encode as $argon2id$v=19$m=<memory>,t=<time>,p=<threads>$<salt>$<hash>
b64Salt := base64.RawStdEncoding.EncodeToString(salt)
b64Hash := base64.RawStdEncoding.EncodeToString(hash)

encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
cfg.Argon2Memory, cfg.Argon2Time, cfg.Argon2Threads, b64Salt, b64Hash)

return encoded, nil
}

// verifyArgon2 verifies an Argon2id hash.
func verifyArgon2(data, encoded string, cfg Config) bool {
// Parse the encoded hash
parts := strings.Split(encoded, "$")
if len(parts) != 6 {
return false
}

var memory, time uint32
var threads uint8
_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)
if err != nil {
return false
}

salt, err := base64.RawStdEncoding.DecodeString(parts[4])
if err != nil {
return false
}

expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
if err != nil {
return false
}

// Compute hash with same parameters
computedHash := argon2.IDKey([]byte(data), salt, time, memory, threads, uint32(len(expectedHash)))

// Constant-time comparison
return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
}

// ComputeAuditHash computes the hash chain for audit entries.
func ComputeAuditHash(prevHash, data string) string {
h := sha256.New()
h.Write([]byte(prevHash))
h.Write([]byte(data))
return hex.EncodeToString(h.Sum(nil))
}

// ExtractKeyPrefix extracts the prefix from a raw key for identification.
func ExtractKeyPrefix(rawKey string) string {
keyData := strings.TrimPrefix(rawKey, KeyPrefix)
if keyData == rawKey || len(keyData) < 8 {
return ""
}
return keyData[:8]
}
