package auth

import (
"context"
"time"
)

// TenantContextKey is the context key for tenant information.
type TenantContextKey struct{}

// ActorContextKey is the context key for authenticated actor.
type ActorContextKey struct{}

// Tenant represents a tenant with its associated metadata.
type Tenant struct {
ID        string    `json:"id"`
Name      string    `json:"name"`
Plan      string    `json:"plan"` // e.g., "free", "pro", "enterprise"
Status    string    `json:"status"` // e.g., "active", "suspended"
CreatedAt time.Time `json:"createdAt"`
}

// APIKey represents a stored API key.
type APIKey struct {
ID          string    `json:"id"`
TenantID    string    `json:"tenantId"`
Name        string    `json:"name"` // Human-readable label
KeyPrefix   string    `json:"keyPrefix"` // First 8 chars for identification
KeyHash     string    `json:"-"` // Hashed key (never exposed)
Scopes      []string  `json:"scopes"` // e.g., ["audit:read", "audit:write"]
RateLimit   int       `json:"rateLimit"` // Per-minute rate limit (0 = default)
ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
CreatedAt   time.Time `json:"createdAt"`
RevokedAt   *time.Time `json:"revokedAt,omitempty"`
Rotated     bool      `json:"rotated"` // True if this key was rotated (old key in grace period)
RotatedFrom *string   `json:"rotatedFrom,omitempty"` // ID of the previous key
}

// Actor represents the authenticated entity making a request.
type Actor struct {
TenantID   string   `json:"tenantId"`
KeyID      string   `json:"keyId"`
KeyName    string   `json:"keyName"`
Scopes     []string `json:"scopes"`
ActorType  string   `json:"actorType"` // "api_key" or "user" (future)
}

// AuditLogEntry represents an authentication-related audit log entry.
type AuditLogEntry struct {
ID        string    `json:"id"`
TenantID  string    `json:"tenantId"`
CorrID    string    `json:"corrId"`
Action    string    `json:"action"` // e.g., "auth.success", "auth.failure", "key.created"
KeyID     string    `json:"keyId,omitempty"`
IPAddress string    `json:"ipAddress,omitempty"`
UserAgent string    `json:"userAgent,omitempty"`
Details   string    `json:"details,omitempty"`
Timestamp time.Time `json:"timestamp"`
PrevHash  string    `json:"prevHash"` // Hash chain for tamper detection
Hash      string    `json:"hash"`
}

// APIKeyStore defines the interface for API key persistence.
type APIKeyStore interface {
// ValidateKey checks if the raw key is valid and returns the associated tenant.
ValidateKey(ctx context.Context, rawKey string) (*Tenant, *APIKey, error)
// CreateKey creates a new API key and returns the raw key (shown once).
CreateKey(ctx context.Context, tenantID string, name string, scopes []string, expiresAt *time.Time) (*APIKey, string, error)
// RotateKey creates a new key and marks the old one for graceful rotation.
RotateKey(ctx context.Context, oldKeyID string) (*APIKey, string, error)
// RevokeKey immediately revokes an API key.
RevokeKey(ctx context.Context, keyID string) error
// ListKeys returns all keys for a tenant.
ListKeys(ctx context.Context, tenantID string) ([]APIKey, error)
// UpdateLastUsed updates the last used timestamp (async-safe).
UpdateLastUsed(ctx context.Context, keyID string) error
}

// TenantStore defines the interface for tenant persistence.
type TenantStore interface {
// GetTenant retrieves a tenant by ID.
GetTenant(ctx context.Context, tenantID string) (*Tenant, error)
// CreateTenant creates a new tenant.
CreateTenant(ctx context.Context, tenant Tenant) error
// UpdateTenantStatus updates tenant status (e.g., suspend).
UpdateTenantStatus(ctx context.Context, tenantID, status string) error
}

// AuthAuditRecorder records authentication audit events.
type AuthAuditRecorder interface {
// Record appends an audit entry.
Record(ctx context.Context, entry AuditLogEntry) error
// Last returns the last audit entry for chain hashing.
Last(ctx context.Context, tenantID string) (AuditLogEntry, error)
}

// Scopes defines available permission scopes.
var Scopes = struct {
AuditRead  string
AuditWrite string
InvoiceRead  string
InvoiceWrite string
AdminRead    string
AdminWrite   string
}{
AuditRead:    "audit:read",
AuditWrite:   "audit:write",
InvoiceRead:  "invoice:read",
InvoiceWrite: "invoice:write",
AdminRead:    "admin:read",
AdminWrite:   "admin:write",
}

// AllScopes returns all available scopes.
func AllScopes() []string {
return []string{
Scopes.AuditRead,
Scopes.AuditWrite,
Scopes.InvoiceRead,
Scopes.InvoiceWrite,
Scopes.AdminRead,
Scopes.AdminWrite,
}
}

// HasScope checks if the actor has the required scope.
func (a *Actor) HasScope(scope string) bool {
for _, s := range a.Scopes {
if s == scope || s == "*" {
return true
}
}
return false
}

// TenantFromContext extracts the tenant from context.
func TenantFromContext(ctx context.Context) (*Tenant, bool) {
tenant, ok := ctx.Value(TenantContextKey{}).(*Tenant)
return tenant, ok
}

// ActorFromContext extracts the actor from context.
func ActorFromContext(ctx context.Context) (*Actor, bool) {
actor, ok := ctx.Value(ActorContextKey{}).(*Actor)
return actor, ok
}

// ContextWithTenant adds tenant to context.
func ContextWithTenant(ctx context.Context, tenant *Tenant) context.Context {
return context.WithValue(ctx, TenantContextKey{}, tenant)
}

// ContextWithActor adds actor to context.
func ContextWithActor(ctx context.Context, actor *Actor) context.Context {
return context.WithValue(ctx, ActorContextKey{}, actor)
}
