package auth

import (
"encoding/json"
"log/slog"
"net/http"
"time"
)

// Handler provides HTTP handlers for authentication endpoints.
type Handler struct {
store  *InMemoryAPIKeyStore
audit  *InMemoryAuthAuditRecorder
cfg    Config
logger *slog.Logger
}

// NewHandler creates a new auth handler.
func NewHandler(store *InMemoryAPIKeyStore, audit *InMemoryAuthAuditRecorder, cfg Config, logger *slog.Logger) *Handler {
if logger == nil {
logger = slog.Default()
}
return &Handler{
store:  store,
audit:  audit,
cfg:    cfg,
logger: logger,
}
}

// CreateAPIKeyRequest is the request body for creating an API key.
type CreateAPIKeyRequest struct {
Name      string    `json:"name"`
Scopes    []string  `json:"scopes"`
ExpiresAt *string   `json:"expiresAt,omitempty"`
}

// CreateAPIKeyResponse is the response for creating an API key.
type CreateAPIKeyResponse struct {
Key    APIKeyInfo `json:"key"`
RawKey string     `json:"rawKey"`
}

// APIKeyInfo is the public representation of an API key.
type APIKeyInfo struct {
ID         string     `json:"id"`
TenantID   string     `json:"tenantId"`
Name       string     `json:"name"`
KeyPrefix  string     `json:"keyPrefix"`
Scopes     []string   `json:"scopes"`
RateLimit  int        `json:"rateLimit,omitempty"`
ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
CreatedAt  time.Time  `json:"createdAt"`
RevokedAt  *time.Time `json:"revokedAt,omitempty"`
Rotated    bool       `json:"rotated,omitempty"`
}

// ListAPIKeysResponse is the response for listing API keys.
type ListAPIKeysResponse struct {
Keys []APIKeyInfo `json:"keys"`
}

// CreateTenantRequest is the request body for creating a tenant.
type CreateTenantRequest struct {
ID   string `json:"id"`
Name string `json:"name"`
Plan string `json:"plan,omitempty"`
}

// CreateTenantResponse is the response for creating a tenant.
type CreateTenantResponse struct {
Tenant     TenantInfo           `json:"tenant"`
InitialKey CreateAPIKeyResponse `json:"initialKey"`
}

// TenantInfo is the public representation of a tenant.
type TenantInfo struct {
ID        string    `json:"id"`
Name      string    `json:"name"`
Plan      string    `json:"plan"`
Status    string    `json:"status"`
CreatedAt time.Time `json:"createdAt"`
}

// CreateAPIKey handles POST /auth/keys
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
corrID := r.Header.Get("X-Correlation-Id")

actor, ok := ActorFromContext(r.Context())
if !ok {
writeJSONError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required", corrID)
return
}

// Check scope
if !actor.HasScope(Scopes.AdminWrite) && !actor.HasScope("*") {
writeJSONError(w, http.StatusForbidden, "INSUFFICIENT_SCOPE", "admin:write scope required", corrID)
return
}

var req CreateAPIKeyRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
writeJSONError(w, http.StatusBadRequest, "BAD_JSON", "Invalid JSON body", corrID)
return
}

// Validate request
if req.Name == "" {
writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required", corrID)
return
}
if len(req.Scopes) == 0 {
writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "at least one scope is required", corrID)
return
}

var expiresAt *time.Time
if req.ExpiresAt != nil {
t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
if err != nil {
writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid expiresAt format", corrID)
return
}
expiresAt = &t
}

key, rawKey, err := h.store.CreateKey(r.Context(), actor.TenantID, req.Name, req.Scopes, expiresAt)
if err != nil {
h.logger.Error("failed to create API key", slog.String("error", err.Error()))
writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create API key", corrID)
return
}

resp := CreateAPIKeyResponse{
Key:    toAPIKeyInfo(key),
RawKey: rawKey,
}

h.logger.Info("API key created",
slog.String("correlationId", corrID),
slog.String("tenantId", actor.TenantID),
slog.String("keyId", key.ID),
slog.String("keyName", key.Name),
)

writeJSON(w, http.StatusCreated, corrID, resp)
}

// ListAPIKeys handles GET /auth/keys
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
corrID := r.Header.Get("X-Correlation-Id")

actor, ok := ActorFromContext(r.Context())
if !ok {
writeJSONError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required", corrID)
return
}

// Check scope
if !actor.HasScope(Scopes.AdminRead) && !actor.HasScope(Scopes.AdminWrite) && !actor.HasScope("*") {
writeJSONError(w, http.StatusForbidden, "INSUFFICIENT_SCOPE", "admin:read scope required", corrID)
return
}

keys, err := h.store.ListKeys(r.Context(), actor.TenantID)
if err != nil {
h.logger.Error("failed to list API keys", slog.String("error", err.Error()))
writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list API keys", corrID)
return
}

infos := make([]APIKeyInfo, len(keys))
for i, k := range keys {
infos[i] = toAPIKeyInfo(&k)
}

writeJSON(w, http.StatusOK, corrID, ListAPIKeysResponse{Keys: infos})
}

// RevokeAPIKey handles DELETE /auth/keys/{keyId}
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request, keyID string) {
corrID := r.Header.Get("X-Correlation-Id")

actor, ok := ActorFromContext(r.Context())
if !ok {
writeJSONError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required", corrID)
return
}

// Check scope
if !actor.HasScope(Scopes.AdminWrite) && !actor.HasScope("*") {
writeJSONError(w, http.StatusForbidden, "INSUFFICIENT_SCOPE", "admin:write scope required", corrID)
return
}

err := h.store.RevokeKey(r.Context(), keyID)
if err != nil {
writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "API key not found", corrID)
return
}

h.logger.Info("API key revoked",
slog.String("correlationId", corrID),
slog.String("tenantId", actor.TenantID),
slog.String("keyId", keyID),
)

w.Header().Set("X-Correlation-Id", corrID)
w.WriteHeader(http.StatusNoContent)
}

// RotateAPIKey handles POST /auth/keys/{keyId}/rotate
func (h *Handler) RotateAPIKey(w http.ResponseWriter, r *http.Request, keyID string) {
corrID := r.Header.Get("X-Correlation-Id")

actor, ok := ActorFromContext(r.Context())
if !ok {
writeJSONError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required", corrID)
return
}

// Check scope
if !actor.HasScope(Scopes.AdminWrite) && !actor.HasScope("*") {
writeJSONError(w, http.StatusForbidden, "INSUFFICIENT_SCOPE", "admin:write scope required", corrID)
return
}

newKey, rawKey, err := h.store.RotateKey(r.Context(), keyID)
if err != nil {
writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "API key not found or cannot be rotated", corrID)
return
}

resp := CreateAPIKeyResponse{
Key:    toAPIKeyInfo(newKey),
RawKey: rawKey,
}

h.logger.Info("API key rotated",
slog.String("correlationId", corrID),
slog.String("tenantId", actor.TenantID),
slog.String("oldKeyId", keyID),
slog.String("newKeyId", newKey.ID),
)

writeJSON(w, http.StatusOK, corrID, resp)
}

// CreateTenant handles POST /auth/tenants
// Note: In production, this would be admin-only or part of onboarding flow
func (h *Handler) CreateTenant(w http.ResponseWriter, r *http.Request) {
corrID := r.Header.Get("X-Correlation-Id")

var req CreateTenantRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
writeJSONError(w, http.StatusBadRequest, "BAD_JSON", "Invalid JSON body", corrID)
return
}

// Validate request
if req.ID == "" {
writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id is required", corrID)
return
}
if req.Name == "" {
writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required", corrID)
return
}

plan := req.Plan
if plan == "" {
plan = "free"
}

tenant := Tenant{
ID:        req.ID,
Name:      req.Name,
Plan:      plan,
Status:    "active",
CreatedAt: time.Now().UTC(),
}

err := h.store.CreateTenant(r.Context(), tenant)
if err != nil {
writeJSONError(w, http.StatusConflict, "CONFLICT", "Tenant already exists", corrID)
return
}

// Create initial admin key with all scopes
key, rawKey, err := h.store.CreateKey(r.Context(), tenant.ID, "Initial Admin Key", AllScopes(), nil)
if err != nil {
h.logger.Error("failed to create initial API key", slog.String("error", err.Error()))
writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create initial API key", corrID)
return
}

resp := CreateTenantResponse{
Tenant: TenantInfo{
ID:        tenant.ID,
Name:      tenant.Name,
Plan:      tenant.Plan,
Status:    tenant.Status,
CreatedAt: tenant.CreatedAt,
},
InitialKey: CreateAPIKeyResponse{
Key:    toAPIKeyInfo(key),
RawKey: rawKey,
},
}

h.logger.Info("tenant created",
slog.String("correlationId", corrID),
slog.String("tenantId", tenant.ID),
slog.String("keyId", key.ID),
)

writeJSON(w, http.StatusCreated, corrID, resp)
}

func toAPIKeyInfo(k *APIKey) APIKeyInfo {
return APIKeyInfo{
ID:         k.ID,
TenantID:   k.TenantID,
Name:       k.Name,
KeyPrefix:  k.KeyPrefix,
Scopes:     k.Scopes,
RateLimit:  k.RateLimit,
ExpiresAt:  k.ExpiresAt,
LastUsedAt: k.LastUsedAt,
CreatedAt:  k.CreatedAt,
RevokedAt:  k.RevokedAt,
Rotated:    k.Rotated,
}
}

func writeJSON(w http.ResponseWriter, status int, corrID string, v any) {
w.Header().Set("Content-Type", "application/json")
if corrID != "" {
w.Header().Set("X-Correlation-Id", corrID)
}
w.WriteHeader(status)
_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, code, message, corrID string) {
w.Header().Set("Content-Type", "application/json")
if corrID != "" {
w.Header().Set("X-Correlation-Id", corrID)
}
w.WriteHeader(status)
_ = json.NewEncoder(w).Encode(AuthError{
Code:      code,
Message:   message,
CorrID:    corrID,
Retryable: false,
})
}
