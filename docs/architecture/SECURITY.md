# セキュリティ設計書

## 現状の課題
- CORS設定が全許可（AllowOriginFunc: func(r *http.Request, origin string) bool { return true }）
- セキュリティヘッダーなし
- 入力バリデーション不十分
- Rate limitingがインメモリ（単一インスタンス前提）

## 必須セキュリティヘッダー

### 実装
```go
// middleware/security.go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
        next.ServeHTTP(w, r)
    })
}
```

## CORS設定（本番用）

```go
corsHandler := cors.New(cors.Options{
    AllowedOrigins:   []string{"https://app.example.com"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Correlation-Id", "X-Tenant-Id"},
    ExposedHeaders:   []string{"X-Request-Id"},
    AllowCredentials: true,
    MaxAge:           300,
})
```

## 入力バリデーション強化

### UBL XMLインジェクション対策
```go
func sanitizeXMLInput(input string) string {
    // XXE対策
    // エンティティ展開の無効化
    // DTD処理の無効化
}
```

### SQLインジェクション対策
- プリペアドステートメントのみ使用
- ORMのパラメータバインディング

## Rate Limiting（分散対応）

### Redis バックエンド
```go
type RedisRateLimiter struct {
    client *redis.Client
    limit  int
    window time.Duration
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
    pipe := r.client.Pipeline()
    incr := pipe.Incr(ctx, key)
    pipe.Expire(ctx, key, r.window)
    _, err := pipe.Exec(ctx)
    if err != nil {
        return false, err
    }
    return incr.Val() <= int64(r.limit), nil
}
```

## 電帳法コンプライアンス

### 監査ログの改ざん防止
- ハッシュチェーンによる整合性検証（実装済み）
- 定期的な整合性チェックジョブ
- 外部バックアップ

### データ保持
- 7年間の保持義務
- 削除不可設計
- タイムスタンプ証明（将来）

## 脆弱性対策チェックリスト

- [ ] OWASP Top 10 対応
- [ ] 依存関係の定期スキャン（Dependabot）
- [ ] SAST（静的解析）導入
- [ ] ペネトレーションテスト
- [ ] セキュリティヘッダー実装
- [ ] CORS設定の制限
- [ ] Rate limiting（Redis対応）
- [ ] API Key のセキュアな管理
