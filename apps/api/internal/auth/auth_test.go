package auth

import (
"context"
"testing"
"time"
)

func TestGenerateAPIKey(t *testing.T) {
rawKey, prefix, err := GenerateAPIKey()
if err != nil {
t.Fatalf("GenerateAPIKey() error = %v", err)
}

if len(rawKey) < 20 {
t.Errorf("rawKey too short: %d", len(rawKey))
}

if len(prefix) != 8 {
t.Errorf("prefix length = %d, want 8", len(prefix))
}

if rawKey[:4] != KeyPrefix {
t.Errorf("rawKey doesn't start with prefix: %s", rawKey[:4])
}
}

func TestHashAndVerifyKey_Bcrypt(t *testing.T) {
cfg := Config{
APIKeyHashAlgorithm: "bcrypt",
BcryptCost:          10, // Lower cost for faster tests
}

rawKey, _, err := GenerateAPIKey()
if err != nil {
t.Fatalf("GenerateAPIKey() error = %v", err)
}

hash, err := HashKey(rawKey, cfg)
if err != nil {
t.Fatalf("HashKey() error = %v", err)
}

if !VerifyKey(rawKey, hash, cfg) {
t.Error("VerifyKey() returned false for valid key")
}

// Test with wrong key
wrongKey := rawKey + "x"
if VerifyKey(wrongKey, hash, cfg) {
t.Error("VerifyKey() returned true for invalid key")
}
}

func TestHashAndVerifyKey_Argon2(t *testing.T) {
cfg := Config{
APIKeyHashAlgorithm: "argon2",
Argon2Time:          1,
Argon2Memory:        16 * 1024, // Lower memory for faster tests
Argon2Threads:       2,
}

rawKey, _, err := GenerateAPIKey()
if err != nil {
t.Fatalf("GenerateAPIKey() error = %v", err)
}

hash, err := HashKey(rawKey, cfg)
if err != nil {
t.Fatalf("HashKey() error = %v", err)
}

if !VerifyKey(rawKey, hash, cfg) {
t.Error("VerifyKey() returned false for valid key")
}

// Test with wrong key
wrongKey := rawKey + "x"
if VerifyKey(wrongKey, hash, cfg) {
t.Error("VerifyKey() returned true for invalid key")
}
}

func TestHashKey_InvalidFormat(t *testing.T) {
cfg := Config{
APIKeyHashAlgorithm: "bcrypt",
BcryptCost:          10,
}

// Key without prefix
_, err := HashKey("invalid-key-without-prefix", cfg)
if err != ErrInvalidKey {
t.Errorf("HashKey() error = %v, want ErrInvalidKey", err)
}
}

func TestExtractKeyPrefix(t *testing.T) {
rawKey, expectedPrefix, _ := GenerateAPIKey()

prefix := ExtractKeyPrefix(rawKey)
if prefix != expectedPrefix {
t.Errorf("ExtractKeyPrefix() = %s, want %s", prefix, expectedPrefix)
}

// Invalid key
prefix = ExtractKeyPrefix("invalid")
if prefix != "" {
t.Errorf("ExtractKeyPrefix() = %s, want empty string", prefix)
}
}

func TestInMemoryAPIKeyStore_CreateAndValidate(t *testing.T) {
cfg := Config{
APIKeyHashAlgorithm: "bcrypt",
BcryptCost:          10,
KeyRotationWindow:   24 * time.Hour,
}
store := NewInMemoryAPIKeyStore(cfg)
ctx := context.Background()

// Create tenant first
tenant := Tenant{
ID:        "test-tenant",
Name:      "Test Tenant",
Plan:      "pro",
Status:    "active",
CreatedAt: time.Now().UTC(),
}
if err := store.CreateTenant(ctx, tenant); err != nil {
t.Fatalf("CreateTenant() error = %v", err)
}

// Create key
key, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"audit:read"}, nil)
if err != nil {
t.Fatalf("CreateKey() error = %v", err)
}

if key.TenantID != "test-tenant" {
t.Errorf("key.TenantID = %s, want test-tenant", key.TenantID)
}
if key.Name != "Test Key" {
t.Errorf("key.Name = %s, want Test Key", key.Name)
}

// Validate key
validatedTenant, validatedKey, err := store.ValidateKey(ctx, rawKey)
if err != nil {
t.Fatalf("ValidateKey() error = %v", err)
}

if validatedTenant.ID != tenant.ID {
t.Errorf("tenant.ID = %s, want %s", validatedTenant.ID, tenant.ID)
}
if validatedKey.ID != key.ID {
t.Errorf("key.ID = %s, want %s", validatedKey.ID, key.ID)
}

// Validate with wrong key
_, _, err = store.ValidateKey(ctx, rawKey+"x")
if err != ErrInvalidAPIKey {
t.Errorf("ValidateKey() error = %v, want ErrInvalidAPIKey", err)
}
}

func TestInMemoryAPIKeyStore_RevokeKey(t *testing.T) {
cfg := Config{
APIKeyHashAlgorithm: "bcrypt",
BcryptCost:          10,
}
store := NewInMemoryAPIKeyStore(cfg)
ctx := context.Background()

// Create tenant and key
tenant := Tenant{ID: "test-tenant", Name: "Test", Plan: "pro", Status: "active", CreatedAt: time.Now().UTC()}
_ = store.CreateTenant(ctx, tenant)
key, rawKey, _ := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"*"}, nil)

// Revoke
if err := store.RevokeKey(ctx, key.ID); err != nil {
t.Fatalf("RevokeKey() error = %v", err)
}

// Key should still be found but marked as revoked
_, validatedKey, err := store.ValidateKey(ctx, rawKey)
if err != nil {
// Key lookup still works, but RevokedAt should be set
t.Fatalf("ValidateKey() after revoke error = %v", err)
}
if validatedKey.RevokedAt == nil {
t.Error("RevokedAt should be set after revocation")
}
}

func TestInMemoryAPIKeyStore_RotateKey(t *testing.T) {
cfg := Config{
APIKeyHashAlgorithm: "bcrypt",
BcryptCost:          10,
KeyRotationWindow:   24 * time.Hour,
}
store := NewInMemoryAPIKeyStore(cfg)
ctx := context.Background()

// Create tenant and key
tenant := Tenant{ID: "test-tenant", Name: "Test", Plan: "pro", Status: "active", CreatedAt: time.Now().UTC()}
_ = store.CreateTenant(ctx, tenant)
oldKey, oldRawKey, _ := store.CreateKey(ctx, "test-tenant", "Old Key", []string{"*"}, nil)

// Rotate
newKey, newRawKey, err := store.RotateKey(ctx, oldKey.ID)
if err != nil {
t.Fatalf("RotateKey() error = %v", err)
}

if newKey.ID == oldKey.ID {
t.Error("new key ID should be different from old key ID")
}

// Both keys should be valid during grace period
_, _, err = store.ValidateKey(ctx, oldRawKey)
if err != nil {
t.Errorf("old key should still be valid during grace period: %v", err)
}

_, _, err = store.ValidateKey(ctx, newRawKey)
if err != nil {
t.Errorf("new key should be valid: %v", err)
}
}

func TestInMemoryAPIKeyStore_ListKeys(t *testing.T) {
cfg := Config{
APIKeyHashAlgorithm: "bcrypt",
BcryptCost:          10,
}
store := NewInMemoryAPIKeyStore(cfg)
ctx := context.Background()

// Create tenant
tenant := Tenant{ID: "test-tenant", Name: "Test", Plan: "pro", Status: "active", CreatedAt: time.Now().UTC()}
_ = store.CreateTenant(ctx, tenant)

// Create multiple keys
_, _, _ = store.CreateKey(ctx, "test-tenant", "Key 1", []string{"audit:read"}, nil)
_, _, _ = store.CreateKey(ctx, "test-tenant", "Key 2", []string{"audit:write"}, nil)

keys, err := store.ListKeys(ctx, "test-tenant")
if err != nil {
t.Fatalf("ListKeys() error = %v", err)
}

if len(keys) != 2 {
t.Errorf("ListKeys() returned %d keys, want 2", len(keys))
}

// Key hashes should not be exposed
for _, k := range keys {
if k.KeyHash != "" {
t.Error("KeyHash should be empty in listed keys")
}
}
}

func TestRateLimiter(t *testing.T) {
rl := NewRateLimiter(3, time.Second)

// First 3 requests should be allowed
for i := 0; i < 3; i++ {
allowed, _ := rl.Allow("test-key")
if !allowed {
t.Errorf("request %d should be allowed", i+1)
}
}

// 4th request should be denied
allowed, retryAfter := rl.Allow("test-key")
if allowed {
t.Error("4th request should be denied")
}
if retryAfter <= 0 {
t.Error("retryAfter should be positive")
}

// Different key should be allowed
allowed, _ = rl.Allow("other-key")
if !allowed {
t.Error("different key should be allowed")
}
}

func TestActor_HasScope(t *testing.T) {
tests := []struct {
name     string
scopes   []string
required string
want     bool
}{
{"exact match", []string{"audit:read", "audit:write"}, "audit:read", true},
{"no match", []string{"audit:read"}, "audit:write", false},
{"wildcard", []string{"*"}, "anything", true},
{"empty scopes", []string{}, "audit:read", false},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
actor := &Actor{Scopes: tt.scopes}
if got := actor.HasScope(tt.required); got != tt.want {
t.Errorf("HasScope(%s) = %v, want %v", tt.required, got, tt.want)
}
})
}
}

func TestComputeAuditHash(t *testing.T) {
hash1 := ComputeAuditHash("", "data1")
hash2 := ComputeAuditHash(hash1, "data2")
hash3 := ComputeAuditHash(hash1, "data2")

if hash1 == hash2 {
t.Error("different data should produce different hashes")
}

if hash2 != hash3 {
t.Error("same inputs should produce same hash")
}

if len(hash1) != 64 {
t.Errorf("hash length = %d, want 64 (SHA-256 hex)", len(hash1))
}
}
