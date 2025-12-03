package auth

import (
"context"
"crypto/rand"
"crypto/sha256"
"encoding/hex"
"encoding/json"
"errors"
"fmt"
"log/slog"
"net/http"
"strings"
"time"
)

// AuthErrors defines authentication error types.
var (
ErrAPIKeyRequired   = errors.New("API key required")
ErrInvalidAPIKey    = errors.New("invalid API key")
ErrKeyExpired       = errors.New("API key expired")
ErrKeyRevoked       = errors.New("API key revoked")
ErrTenantSuspended  = errors.New("tenant suspended")
ErrInsufficientScope = errors.New("insufficient scope")
)

// AuthError represents an authentication error response.
type AuthError struct {
Code      string `json:"code"`
Message   string `json:"message"`
CorrID    string `json:"corrId"`
Retryable bool   `json:"retryable"`
}

// Middleware creates the API Key authentication middleware.
func Middleware(store APIKeyStore, audit AuthAuditRecorder, cfg Config, logger *slog.Logger) func(http.Handler) http.Handler {
return func(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
corrID := r.Header.Get("X-Correlation-Id")
if corrID == "" {
corrID = generateCorrID()
}

// Extract API key from Authorization header
rawKey := extractAPIKey(r)
if rawKey == "" {
// Also check X-API-Key header for backward compatibility
rawKey = r.Header.Get("X-API-Key")
}

if rawKey == "" {
writeAuthError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "API key required", corrID, false)
recordAuthFailure(r.Context(), audit, "", corrID, "auth.missing_key", r)
return
}

// Validate the key
tenant, apiKey, err := store.ValidateKey(r.Context(), rawKey)
if err != nil {
handleAuthError(w, r, audit, cfg, corrID, rawKey, err)
return
}

// Check tenant status
if tenant.Status != "active" {
writeAuthError(w, http.StatusForbidden, "TENANT_SUSPENDED", "Tenant account is suspended", corrID, false)
recordAuthFailure(r.Context(), audit, tenant.ID, corrID, "auth.tenant_suspended", r)
return
}

// Check key expiration
if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
// Check rotation grace period
if apiKey.Rotated {
gracePeriod := time.Now().Add(-cfg.KeyRotationWindow)
if apiKey.ExpiresAt.Before(gracePeriod) {
writeAuthError(w, http.StatusUnauthorized, "KEY_EXPIRED", "API key has expired", corrID, false)
recordAuthFailure(r.Context(), audit, tenant.ID, corrID, "auth.key_expired", r)
return
}
} else {
writeAuthError(w, http.StatusUnauthorized, "KEY_EXPIRED", "API key has expired", corrID, false)
recordAuthFailure(r.Context(), audit, tenant.ID, corrID, "auth.key_expired", r)
return
}
}

// Check revocation
if apiKey.RevokedAt != nil {
writeAuthError(w, http.StatusUnauthorized, "KEY_REVOKED", "API key has been revoked", corrID, false)
recordAuthFailure(r.Context(), audit, tenant.ID, corrID, "auth.key_revoked", r)
return
}

// Build actor
actor := &Actor{
TenantID:  tenant.ID,
KeyID:     apiKey.ID,
KeyName:   apiKey.Name,
Scopes:    apiKey.Scopes,
ActorType: "api_key",
}

// Update last used (fire and forget)
go func() {
    if err := store.UpdateLastUsed(context.Background(), apiKey.ID); err != nil {
        if logger != nil {
            logger.Error("Failed to update last used for API key", "keyID", apiKey.ID, "error", err)
        } else {
            slog.Error("Failed to update last used for API key", "keyID", apiKey.ID, "error", err)
        }
    }
}()

// Record success
if cfg.EnableAuditLog && audit != nil {
recordAuthSuccess(r.Context(), audit, tenant.ID, corrID, apiKey.ID, r)
}

// Add to context and continue
ctx := r.Context()
ctx = ContextWithTenant(ctx, tenant)
ctx = ContextWithActor(ctx, actor)

// Log successful auth
if logger != nil {
logger.Info("authenticated request",
slog.String("correlationId", corrID),
slog.String("tenantId", tenant.ID),
slog.String("keyId", apiKey.ID),
slog.String("keyName", apiKey.Name),
)
}

next.ServeHTTP(w, r.WithContext(ctx))
})
}
}

// RequireScope creates middleware that enforces a specific scope.
func RequireScope(scope string) func(http.Handler) http.Handler {
return func(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
actor, ok := ActorFromContext(r.Context())
if !ok {
writeAuthError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required", "", false)
return
}

if !actor.HasScope(scope) {
corrID := r.Header.Get("X-Correlation-Id")
writeAuthError(w, http.StatusForbidden, "INSUFFICIENT_SCOPE", 
fmt.Sprintf("Required scope: %s", scope), corrID, false)
return
}

next.ServeHTTP(w, r)
})
}
}

// extractAPIKey extracts the API key from the Authorization header.
// Supports: Bearer <key>, ApiKey <key>, or just <key>
func extractAPIKey(r *http.Request) string {
auth := r.Header.Get("Authorization")
if auth == "" {
return ""
}

// Handle "Bearer <key>"
if strings.HasPrefix(auth, "Bearer ") {
return strings.TrimPrefix(auth, "Bearer ")
}

// Handle "ApiKey <key>"
if strings.HasPrefix(auth, "ApiKey ") {
return strings.TrimPrefix(auth, "ApiKey ")
}

// Handle raw key (less common)
return auth
}

func handleAuthError(w http.ResponseWriter, r *http.Request, audit AuthAuditRecorder, cfg Config, corrID, rawKey string, err error) {
keyPrefix := ExtractKeyPrefix(rawKey)

switch {
case errors.Is(err, ErrInvalidKey):
writeAuthError(w, http.StatusUnauthorized, "INVALID_KEY", "Invalid API key format", corrID, false)
recordAuthFailure(r.Context(), audit, "", corrID, "auth.invalid_format", r)
case errors.Is(err, ErrInvalidAPIKey):
writeAuthError(w, http.StatusUnauthorized, "INVALID_KEY", "Invalid API key", corrID, false)
recordAuthFailure(r.Context(), audit, "", corrID, "auth.invalid_key", r)
default:
writeAuthError(w, http.StatusUnauthorized, "AUTH_FAILED", "Authentication failed", corrID, false)
recordAuthFailure(r.Context(), audit, "", corrID, "auth.failed", r)
}

_ = keyPrefix // Could log this for debugging
}

func writeAuthError(w http.ResponseWriter, status int, code, message, corrID string, retryable bool) {
w.Header().Set("Content-Type", "application/json")
if corrID != "" {
w.Header().Set("X-Correlation-Id", corrID)
}
w.WriteHeader(status)

resp := AuthError{
Code:      code,
Message:   message,
CorrID:    corrID,
Retryable: retryable,
}
_ = json.NewEncoder(w).Encode(resp)
}

func recordAuthFailure(ctx context.Context, audit AuthAuditRecorder, tenantID, corrID, action string, r *http.Request) {
if audit == nil {
return
}

entry := AuditLogEntry{
ID:        generateID(),
TenantID:  tenantID,
CorrID:    corrID,
Action:    action,
IPAddress: getClientIP(r),
UserAgent: r.UserAgent(),
Timestamp: time.Now().UTC(),
}

// Get previous hash for chain
if tenantID != "" {
if prev, err := audit.Last(ctx, tenantID); err == nil {
entry.PrevHash = prev.Hash
}
}

// Compute hash
data := fmt.Sprintf("%s|%s|%s|%s|%s", entry.ID, entry.TenantID, entry.Action, entry.Timestamp.Format(time.RFC3339), entry.PrevHash)
entry.Hash = ComputeAuditHash(entry.PrevHash, data)

_ = audit.Record(ctx, entry)
}

func recordAuthSuccess(ctx context.Context, audit AuthAuditRecorder, tenantID, corrID, keyID string, r *http.Request) {
if audit == nil {
return
}

entry := AuditLogEntry{
ID:        generateID(),
TenantID:  tenantID,
CorrID:    corrID,
Action:    "auth.success",
KeyID:     keyID,
IPAddress: getClientIP(r),
UserAgent: r.UserAgent(),
Timestamp: time.Now().UTC(),
}

// Get previous hash for chain
if prev, err := audit.Last(ctx, tenantID); err == nil {
entry.PrevHash = prev.Hash
}

// Compute hash
data := fmt.Sprintf("%s|%s|%s|%s|%s", entry.ID, entry.TenantID, entry.Action, entry.Timestamp.Format(time.RFC3339), entry.PrevHash)
entry.Hash = ComputeAuditHash(entry.PrevHash, data)

_ = audit.Record(ctx, entry)
}

func getClientIP(r *http.Request) string {
// Check X-Forwarded-For first (for proxies)
if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
parts := strings.Split(xff, ",")
return strings.TrimSpace(parts[0])
}

// Check X-Real-IP
if xri := r.Header.Get("X-Real-IP"); xri != "" {
return xri
}

// Fall back to RemoteAddr
return r.RemoteAddr
}

func generateCorrID() string {
b := make([]byte, 16)
_, _ = rand.Read(b)
return hex.EncodeToString(b)
}

func generateID() string {
b := make([]byte, 16)
_, _ = rand.Read(b)
h := sha256.Sum256(b)
return hex.EncodeToString(h[:16])
}
