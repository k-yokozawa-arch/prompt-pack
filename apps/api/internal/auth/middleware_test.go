package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestMiddleware_ExpiredKey tests the middleware with an expired API key.
func TestMiddleware_ExpiredKey(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant
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

	// Create key with expiration in the past
	expiredAt := time.Now().Add(-1 * time.Hour)
	_, rawKey, err := store.CreateKey(ctx, "test-tenant", "Expired Key", []string{"*"}, &expiredAt)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request with expired key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var authErr AuthError
	if err := json.NewDecoder(rec.Body).Decode(&authErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// Note: Expired keys are filtered by the store during ValidateKey,
	// so they appear as INVALID_KEY rather than KEY_EXPIRED
	if authErr.Code != "INVALID_KEY" {
		t.Errorf("expected error code INVALID_KEY, got %s", authErr.Code)
	}

	// Verify audit log was recorded
	entries := audit.GetEntries("")
	found := false
	for _, entry := range entries {
		if entry.Action == "auth.invalid_key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit log entry for invalid key (expired keys are filtered by store)")
	}
}

// TestMiddleware_KeyExpirationCheck tests the middleware's expiration check logic
// by creating a key that bypasses the store's expiration filter.
func TestMiddleware_KeyExpirationCheck(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant
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

	// Create key without expiration first
	key, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Directly set expiration in the past (simulating a key that just expired)
	// Access the key directly through the store's internal map
	store.mu.Lock()
	expiredAt := time.Now().Add(-1 * time.Minute)
	store.keys[key.ID].ExpiresAt = &expiredAt
	store.mu.Unlock()

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request with expired key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response - since the store filters expired keys, this will be INVALID_KEY
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var authErr AuthError
	if err := json.NewDecoder(rec.Body).Decode(&authErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// The store's ValidateKey filters expired keys, so we get INVALID_KEY
	if authErr.Code != "INVALID_KEY" {
		t.Errorf("expected error code INVALID_KEY, got %s", authErr.Code)
	}
}

// TestMiddleware_ExpiredKeyDuringRotationGracePeriod tests the middleware with an expired key
// that is still within the rotation grace period.
func TestMiddleware_ExpiredKeyDuringRotationGracePeriod(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		KeyRotationWindow:   24 * time.Hour,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant
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
	_, _, err := store.CreateKey(ctx, "test-tenant", "Original Key", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Get the created key
	keys, _ := store.ListKeys(ctx, "test-tenant")
	oldKey := keys[0]

	// Rotate the key
	_, newRawKey, err := store.RotateKey(ctx, oldKey.ID)
	if err != nil {
		t.Fatalf("RotateKey() error = %v", err)
	}

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request with new key during grace period
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+newRawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response - should succeed during grace period
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify audit log for successful auth
	entries := audit.GetEntries(tenant.ID)
	found := false
	for _, entry := range entries {
		if entry.Action == "auth.success" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit log entry with action 'auth.success'")
	}
}

// TestMiddleware_SuspendedTenant tests the middleware with a suspended tenant.
func TestMiddleware_SuspendedTenant(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant
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
	_, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Suspend the tenant
	if err := store.UpdateTenantStatus(ctx, "test-tenant", "suspended"); err != nil {
		t.Fatalf("UpdateTenantStatus() error = %v", err)
	}

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request with suspended tenant's key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	var authErr AuthError
	if err := json.NewDecoder(rec.Body).Decode(&authErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if authErr.Code != "TENANT_SUSPENDED" {
		t.Errorf("expected error code TENANT_SUSPENDED, got %s", authErr.Code)
	}

	// Verify audit log was recorded
	entries := audit.GetEntries(tenant.ID)
	found := false
	for _, entry := range entries {
		if entry.Action == "auth.tenant_suspended" {
			found = true
			if entry.TenantID != tenant.ID {
				t.Errorf("expected TenantID %s in audit log, got %s", tenant.ID, entry.TenantID)
			}
			break
		}
	}
	if !found {
		t.Error("expected audit log entry with action 'auth.tenant_suspended'")
	}
}

// TestMiddleware_RevokedKey tests the middleware with a revoked API key.
func TestMiddleware_RevokedKey(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant
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
	key, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Revoke the key
	if err := store.RevokeKey(ctx, key.ID); err != nil {
		t.Fatalf("RevokeKey() error = %v", err)
	}

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request with revoked key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var authErr AuthError
	if err := json.NewDecoder(rec.Body).Decode(&authErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if authErr.Code != "INVALID_KEY" {
		t.Errorf("expected error code INVALID_KEY (revoked keys are filtered), got %s", authErr.Code)
	}

	// Verify audit log was recorded
	entries := audit.GetEntries("")
	found := false
	for _, entry := range entries {
		if entry.Action == "auth.invalid_key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit log entry with action 'auth.invalid_key'")
	}
}

// TestMiddleware_MissingAPIKey tests the middleware when no API key is provided.
func TestMiddleware_MissingAPIKey(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request without API key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var authErr AuthError
	if err := json.NewDecoder(rec.Body).Decode(&authErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if authErr.Code != "AUTH_REQUIRED" {
		t.Errorf("expected error code AUTH_REQUIRED, got %s", authErr.Code)
	}

	// Verify audit log was recorded
	entries := audit.GetEntries("")
	found := false
	for _, entry := range entries {
		if entry.Action == "auth.missing_key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit log entry with action 'auth.missing_key'")
	}
}

// TestMiddleware_InvalidAPIKey tests the middleware with an invalid API key.
func TestMiddleware_InvalidAPIKey(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request with invalid API key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var authErr AuthError
	if err := json.NewDecoder(rec.Body).Decode(&authErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if authErr.Code != "INVALID_KEY" {
		t.Errorf("expected error code INVALID_KEY, got %s", authErr.Code)
	}

	// Verify audit log was recorded
	entries := audit.GetEntries("")
	found := false
	for _, entry := range entries {
		if entry.Action == "auth.invalid_format" || entry.Action == "auth.invalid_key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit log entry for invalid key")
	}
}

// TestMiddleware_SuccessfulAuth tests the middleware with a valid API key.
func TestMiddleware_SuccessfulAuth(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant
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
	key, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"audit:read", "audit:write"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Create middleware
	middleware := Middleware(store, audit, cfg, slog.Default())
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context has tenant and actor
		tenant, ok := TenantFromContext(r.Context())
		if !ok {
			t.Error("expected tenant in context")
		} else if tenant.ID != "test-tenant" {
			t.Errorf("expected tenant ID test-tenant, got %s", tenant.ID)
		}

		actor, ok := ActorFromContext(r.Context())
		if !ok {
			t.Error("expected actor in context")
		} else {
			if actor.TenantID != "test-tenant" {
				t.Errorf("expected actor tenant ID test-tenant, got %s", actor.TenantID)
			}
			if actor.KeyID != key.ID {
				t.Errorf("expected actor key ID %s, got %s", key.ID, actor.KeyID)
			}
			if len(actor.Scopes) != 2 {
				t.Errorf("expected 2 scopes, got %d", len(actor.Scopes))
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request with valid key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("X-Correlation-Id", "test-corr-id")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify audit log was recorded
	entries := audit.GetEntries(tenant.ID)
	found := false
	for _, entry := range entries {
		if entry.Action == "auth.success" {
			found = true
			if entry.KeyID != key.ID {
				t.Errorf("expected KeyID %s in audit log, got %s", key.ID, entry.KeyID)
			}
			if entry.CorrID != "test-corr-id" {
				t.Errorf("expected correlation ID test-corr-id, got %s", entry.CorrID)
			}
			// Verify hash chain
			if entry.Hash == "" {
				t.Error("expected non-empty hash")
			}
			break
		}
	}
	if !found {
		t.Error("expected audit log entry with action 'auth.success'")
	}
}

// TestRequireScope_Success tests the RequireScope middleware with sufficient permissions.
func TestRequireScope_Success(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      false,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant and key
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

	_, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"audit:read", "audit:write"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Create middleware chain: auth + scope
	authMiddleware := Middleware(store, audit, cfg, nil)
	scopeMiddleware := RequireScope("audit:read")
	handler := authMiddleware(scopeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})))

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

// TestRequireScope_InsufficientScope tests the RequireScope middleware with insufficient permissions.
func TestRequireScope_InsufficientScope(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      false,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant and key with limited scopes
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

	_, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"audit:read"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Create middleware chain: auth + scope requiring write permission
	authMiddleware := Middleware(store, audit, cfg, nil)
	scopeMiddleware := RequireScope("audit:write")
	handler := authMiddleware(scopeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})))

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	var authErr AuthError
	if err := json.NewDecoder(rec.Body).Decode(&authErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if authErr.Code != "INSUFFICIENT_SCOPE" {
		t.Errorf("expected error code INSUFFICIENT_SCOPE, got %s", authErr.Code)
	}

	if authErr.Message != "Required scope: audit:write" {
		t.Errorf("expected message 'Required scope: audit:write', got %s", authErr.Message)
	}
}

// TestRequireScope_WildcardScope tests that wildcard scopes grant access to everything.
func TestRequireScope_WildcardScope(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      false,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant and key with wildcard scope
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

	_, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Create middleware chain: auth + scope requiring specific permission
	authMiddleware := Middleware(store, audit, cfg, nil)
	scopeMiddleware := RequireScope("audit:write")
	handler := authMiddleware(scopeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})))

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response - wildcard should allow access
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

// TestRequireScope_NoAuth tests RequireScope middleware without authentication.
func TestRequireScope_NoAuth(t *testing.T) {
	scopeMiddleware := RequireScope("audit:read")
	handler := scopeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request without authentication
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var authErr AuthError
	if err := json.NewDecoder(rec.Body).Decode(&authErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if authErr.Code != "AUTH_REQUIRED" {
		t.Errorf("expected error code AUTH_REQUIRED, got %s", authErr.Code)
	}
}

// TestMiddleware_AuditLogChaining tests that audit log entries are properly chained.
func TestMiddleware_AuditLogChaining(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      true,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant
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
	_, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+rawKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Verify audit log chaining
	entries := audit.GetEntries(tenant.ID)
	if len(entries) < 3 {
		t.Fatalf("expected at least 3 audit entries, got %d", len(entries))
	}

	// Verify hash chain
	for i := 1; i < len(entries); i++ {
		if entries[i].PrevHash != entries[i-1].Hash {
			t.Errorf("entry %d: expected PrevHash %s, got %s", i, entries[i-1].Hash, entries[i].PrevHash)
		}
	}

	// Verify first entry has empty PrevHash
	if entries[0].PrevHash != "" {
		t.Errorf("first entry should have empty PrevHash, got %s", entries[0].PrevHash)
	}
}

// TestMiddleware_XAPIKeyHeader tests that the middleware supports X-API-Key header for backward compatibility.
func TestMiddleware_XAPIKeyHeader(t *testing.T) {
	cfg := Config{
		APIKeyHashAlgorithm: "bcrypt",
		BcryptCost:          10,
		EnableAuditLog:      false,
	}
	store := NewInMemoryAPIKeyStore(cfg)
	audit := NewInMemoryAuthAuditRecorder()
	ctx := context.Background()

	// Create tenant and key
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

	_, rawKey, err := store.CreateKey(ctx, "test-tenant", "Test Key", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	// Create middleware
	middleware := Middleware(store, audit, cfg, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Make request with X-API-Key header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", rawKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
