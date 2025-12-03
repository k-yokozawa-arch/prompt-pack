# 認証・認可設計書

## 現状
- X-Tenant-Id ヘッダーによる簡易テナント識別のみ
- 認証・認可機構なし

## 目標アーキテクチャ

### Phase 1: API Key認証（MVP+）
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│   Gateway   │────▶│   API       │
│             │     │  (API Key)  │     │   Server    │
└─────────────┘     └─────────────┘     └─────────────┘
```

### Phase 2: OAuth 2.0 / OIDC（Production）
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│   Auth0/    │────▶│   API       │
│             │     │   Cognito   │     │   Server    │
└─────────────┘     └─────────────┘     └─────────────┘
```

## 実装計画

### API Key認証
```go
// middleware/auth.go
func APIKeyAuth(store APIKeyStore) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := r.Header.Get("X-API-Key")
            if key == "" {
                http.Error(w, "API key required", http.StatusUnauthorized)
                return
            }
            
            tenant, err := store.ValidateKey(r.Context(), key)
            if err != nil {
                http.Error(w, "Invalid API key", http.StatusUnauthorized)
                return
            }
            
            ctx := context.WithValue(r.Context(), TenantKey, tenant)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### テナント分離
- Row-Level Security (RLS) をPostgreSQLで実装
- 全クエリにtenant_id条件を自動付与
- マルチテナント対応のストレージパス設計

## セキュリティ要件
- [ ] API Key のハッシュ保存（bcrypt/argon2）
- [ ] Rate limiting per tenant
- [ ] Key rotation機能
- [ ] Audit log for auth events
