package auth

import (
"context"
"fmt"
"sync"
"time"
)

// InMemoryAPIKeyStore provides an in-memory implementation of APIKeyStore.
// For production, replace with PostgreSQL/Redis implementation.
type InMemoryAPIKeyStore struct {
mu       sync.RWMutex
cfg      Config
keys     map[string]*APIKey      // keyID -> APIKey
keyHash  map[string]string       // keyHash -> keyID (for lookup)
tenants  map[string]*Tenant      // tenantID -> Tenant
}

// NewInMemoryAPIKeyStore creates a new in-memory API key store.
func NewInMemoryAPIKeyStore(cfg Config) *InMemoryAPIKeyStore {
return &InMemoryAPIKeyStore{
cfg:     cfg,
keys:    make(map[string]*APIKey),
keyHash: make(map[string]string),
tenants: make(map[string]*Tenant),
}
}

// ValidateKey validates a raw API key and returns the tenant.
func (s *InMemoryAPIKeyStore) ValidateKey(ctx context.Context, rawKey string) (*Tenant, *APIKey, error) {
s.mu.RLock()
defer s.mu.RUnlock()

// Search through all keys (not efficient for production)
for _, key := range s.keys {
if VerifyKey(rawKey, key.KeyHash, s.cfg) {
tenant, ok := s.tenants[key.TenantID]
if !ok {
return nil, nil, ErrInvalidAPIKey
}
return tenant, key, nil
}
}

return nil, nil, ErrInvalidAPIKey
}

// CreateKey creates a new API key.
func (s *InMemoryAPIKeyStore) CreateKey(ctx context.Context, tenantID, name string, scopes []string, expiresAt *time.Time) (*APIKey, string, error) {
s.mu.Lock()
defer s.mu.Unlock()

// Check tenant exists
if _, ok := s.tenants[tenantID]; !ok {
return nil, "", fmt.Errorf("tenant not found: %s", tenantID)
}

// Generate key
rawKey, prefix, err := GenerateAPIKey()
if err != nil {
return nil, "", err
}

// Hash key
hash, err := HashKey(rawKey, s.cfg)
if err != nil {
return nil, "", err
}

keyID := generateID()
now := time.Now().UTC()

key := &APIKey{
ID:        keyID,
TenantID:  tenantID,
Name:      name,
KeyPrefix: prefix,
KeyHash:   hash,
Scopes:    scopes,
ExpiresAt: expiresAt,
CreatedAt: now,
}

s.keys[keyID] = key
s.keyHash[hash] = keyID

return key, rawKey, nil
}

// RotateKey creates a new key and marks the old one for rotation.
func (s *InMemoryAPIKeyStore) RotateKey(ctx context.Context, oldKeyID string) (*APIKey, string, error) {
s.mu.Lock()
defer s.mu.Unlock()

oldKey, ok := s.keys[oldKeyID]
if !ok {
return nil, "", fmt.Errorf("key not found: %s", oldKeyID)
}

if oldKey.RevokedAt != nil {
return nil, "", fmt.Errorf("cannot rotate revoked key")
}

// Generate new key
rawKey, prefix, err := GenerateAPIKey()
if err != nil {
return nil, "", err
}

hash, err := HashKey(rawKey, s.cfg)
if err != nil {
return nil, "", err
}

newKeyID := generateID()
now := time.Now().UTC()
expiresAt := now.Add(s.cfg.KeyRotationWindow)

// Mark old key as rotated with grace period
oldKey.Rotated = true
oldKey.ExpiresAt = &expiresAt

// Create new key
newKey := &APIKey{
ID:          newKeyID,
TenantID:    oldKey.TenantID,
Name:        oldKey.Name + " (rotated)",
KeyPrefix:   prefix,
KeyHash:     hash,
Scopes:      oldKey.Scopes,
RateLimit:   oldKey.RateLimit,
CreatedAt:   now,
RotatedFrom: &oldKeyID,
}

s.keys[newKeyID] = newKey
s.keyHash[hash] = newKeyID

return newKey, rawKey, nil
}

// RevokeKey revokes an API key immediately.
func (s *InMemoryAPIKeyStore) RevokeKey(ctx context.Context, keyID string) error {
s.mu.Lock()
defer s.mu.Unlock()

key, ok := s.keys[keyID]
if !ok {
return fmt.Errorf("key not found: %s", keyID)
}

now := time.Now().UTC()
key.RevokedAt = &now
return nil
}

// ListKeys returns all keys for a tenant.
func (s *InMemoryAPIKeyStore) ListKeys(ctx context.Context, tenantID string) ([]APIKey, error) {
s.mu.RLock()
defer s.mu.RUnlock()

var keys []APIKey
for _, key := range s.keys {
if key.TenantID == tenantID {
// Return copy without hash
keyCopy := *key
keyCopy.KeyHash = ""
keys = append(keys, keyCopy)
}
}
return keys, nil
}

// UpdateLastUsed updates the last used timestamp.
func (s *InMemoryAPIKeyStore) UpdateLastUsed(ctx context.Context, keyID string) error {
s.mu.Lock()
defer s.mu.Unlock()

key, ok := s.keys[keyID]
if !ok {
return nil // Silently ignore
}

now := time.Now().UTC()
key.LastUsedAt = &now
return nil
}

// CreateTenant creates a new tenant.
func (s *InMemoryAPIKeyStore) CreateTenant(ctx context.Context, tenant Tenant) error {
s.mu.Lock()
defer s.mu.Unlock()

if _, ok := s.tenants[tenant.ID]; ok {
return fmt.Errorf("tenant already exists: %s", tenant.ID)
}

s.tenants[tenant.ID] = &tenant
return nil
}

// GetTenant retrieves a tenant by ID.
func (s *InMemoryAPIKeyStore) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
s.mu.RLock()
defer s.mu.RUnlock()

tenant, ok := s.tenants[tenantID]
if !ok {
return nil, fmt.Errorf("tenant not found: %s", tenantID)
}
return tenant, nil
}

// UpdateTenantStatus updates a tenant's status.
func (s *InMemoryAPIKeyStore) UpdateTenantStatus(ctx context.Context, tenantID, status string) error {
s.mu.Lock()
defer s.mu.Unlock()

tenant, ok := s.tenants[tenantID]
if !ok {
return fmt.Errorf("tenant not found: %s", tenantID)
}

tenant.Status = status
return nil
}

// --- In-memory Audit Recorder ---

// InMemoryAuthAuditRecorder provides an in-memory audit log implementation.
type InMemoryAuthAuditRecorder struct {
mu       sync.RWMutex
entries  map[string][]AuditLogEntry // tenantID -> entries
}

// NewInMemoryAuthAuditRecorder creates a new in-memory audit recorder.
func NewInMemoryAuthAuditRecorder() *InMemoryAuthAuditRecorder {
return &InMemoryAuthAuditRecorder{
entries: make(map[string][]AuditLogEntry),
}
}

// Record appends an audit entry.
func (r *InMemoryAuthAuditRecorder) Record(ctx context.Context, entry AuditLogEntry) error {
r.mu.Lock()
defer r.mu.Unlock()

r.entries[entry.TenantID] = append(r.entries[entry.TenantID], entry)
return nil
}

// Last returns the last audit entry for a tenant.
func (r *InMemoryAuthAuditRecorder) Last(ctx context.Context, tenantID string) (AuditLogEntry, error) {
r.mu.RLock()
defer r.mu.RUnlock()

entries := r.entries[tenantID]
if len(entries) == 0 {
return AuditLogEntry{}, fmt.Errorf("no entries")
}
return entries[len(entries)-1], nil
}

// GetEntries returns all entries for a tenant (for debugging).
func (r *InMemoryAuthAuditRecorder) GetEntries(tenantID string) []AuditLogEntry {
r.mu.RLock()
defer r.mu.RUnlock()

return append([]AuditLogEntry{}, r.entries[tenantID]...)
}
