# 品質ルーブリック（自己レビュー基準）

PRを出す前に、以下の観点をすべてチェックしてください。
満たしていない項目があれば、**最小diffを添えて修正**すること。

---

## 1. 保守運用性 (Maintainability)

### 1.1 設定外出し
- [ ] **ハードコード禁止**: URL、APIキー、閾値、タイムアウト値は全て環境変数から取得
- [ ] **設定モジュール**: `config.go` / `config.ts` で一元管理
- [ ] **デフォルト値**: 環境変数未設定時の安全なフォールバック
- [ ] **バリデーション**: 起動時に必須設定の存在チェック

```go
// ❌ Bad
timeout := 30 * time.Second

// ✅ Good
timeout := cfg.RequestTimeout // from env: REQUEST_TIMEOUT_SECONDS
```

### 1.2 構造化ログ
- [ ] **必須フィールド**: `timestamp`, `level`, `message`, `correlationId`, `tenantId`
- [ ] **JSON形式**: 本番環境ではJSON出力
- [ ] **PIIマスク**: 個人情報（メール、電話、氏名）はマスク or 除外
- [ ] **エラー詳細**: スタックトレースはERRORレベルのみ

```go
// ✅ Good
slog.Info("invoice created",
    slog.String("correlationId", corrId),
    slog.String("tenantId", tenantId),
    slog.String("invoiceId", id),
    slog.String("action", "invoice.created"),
)
```

### 1.3 タイムアウト・リトライ
- [ ] **HTTPクライアント**: 全外部呼び出しにタイムアウト設定（推奨: 30秒以下）
- [ ] **DBクエリ**: コンテキストタイムアウト付与
- [ ] **リトライ**: 一時的エラーは指数バックオフでリトライ（最大3回）
- [ ] **サーキットブレーカー**: 連続失敗時の自動遮断

### 1.4 相関ID (Correlation ID)
- [ ] **受信**: `X-Correlation-Id` ヘッダーから取得
- [ ] **生成**: 未設定時はUUID生成
- [ ] **伝播**: 全ログ・下流呼び出しに付与
- [ ] **レスポンス**: レスポンスヘッダーにも返却

### 1.5 監視・メトリクス
- [ ] **ヘルスチェック**: `/health` エンドポイント（DB/外部依存の死活）
- [ ] **メトリクス**: `/metrics` (Prometheus形式)
  - `http_requests_total{method, path, status}`
  - `http_request_duration_seconds{method, path}`
  - `business_events_total{type}` (invoice_created等)
- [ ] **アラート閾値**: エラー率>1%, P99>1s

---

## 2. UI/UX (Frontend)

### 2.1 アクセシビリティ (a11y)
- [ ] **セマンティックHTML**: `<main>`, `<nav>`, `<article>`, `<section>`
- [ ] **ラベル**: 全入力に `<label>` または `aria-label`
- [ ] **フォーカスリング**: キーボードナビゲーション可視化（`focus-visible`）
- [ ] **コントラスト比**: テキスト 4.5:1以上、大文字 3:1以上
- [ ] **スクリーンリーダー**: `aria-live` で動的更新を通知
- [ ] **キーボード操作**: Tab/Enter/Escapeで全機能アクセス可能

### 2.2 UI状態（4状態必須）
- [ ] **Loading**: スケルトン or スピナー（操作ブロック時はオーバーレイ）
- [ ] **Empty**: データなし時のCTA（「最初の○○を作成」）
- [ ] **Error**: エラーメッセージ + リトライボタン + サポート導線
- [ ] **Success**: トースト通知（自動消去5秒 + 手動消去）

```tsx
// ✅ Good: 4状態を網羅
if (isLoading) return <Skeleton />
if (error) return <ErrorState onRetry={refetch} />
if (data.length === 0) return <EmptyState onCreate={handleCreate} />
return <DataList data={data} />
```

### 2.3 レスポンシブ
- [ ] **ブレークポイント**: mobile (<640px), tablet (640-1024px), desktop (>1024px)
- [ ] **タッチターゲット**: 最小44x44px
- [ ] **横スクロール禁止**: 全画面幅で収まる
- [ ] **フォントサイズ**: 最小14px（モバイル16px推奨）

### 2.4 フォーム
- [ ] **バリデーション**: リアルタイム + 送信時
- [ ] **エラー表示**: フィールド直下に赤文字
- [ ] **送信中状態**: ボタン無効化 + スピナー
- [ ] **二重送信防止**: デバウンス or 状態管理

---

## 3. セキュリティ (Security)

### 3.1 入力検証
- [ ] **スキーマ検証**: 全APIエンドポイントでJSON Schema / Zod / oapi-codegen
- [ ] **型強制**: 文字列→数値の暗黙変換禁止
- [ ] **長さ制限**: 全文字列フィールドに最大長
- [ ] **形式検証**: email, URL, UUID, 日付は形式チェック
- [ ] **SQLインジェクション**: プリペアドステートメントのみ
- [ ] **XSS**: 出力時エスケープ（React自動、dangerouslySetInnerHTML禁止）

```go
// ✅ Good: OpenAPI生成型 + 追加バリデーション
func (s *Service) CreateInvoice(ctx context.Context, req CreateInvoiceRequest) error {
    if err := req.Validate(); err != nil {  // 生成されたバリデーション
        return ErrValidation
    }
    if len(req.Lines) == 0 {  // ビジネスルール
        return ErrNoLines
    }
}
```

### 3.2 認証・認可
- [ ] **認証必須**: 公開API以外は全て認証チェック
- [ ] **テナント分離**: 全クエリに `tenant_id` 条件（RLS推奨）
- [ ] **IDOR対策**: リソースアクセス時にオーナーシップ検証
- [ ] **権限チェック**: エンドポイント毎にロール/スコープ確認
- [ ] **403 vs 404**: 存在しない→404、権限なし→403（情報漏洩注意）

### 3.3 監査ログ
- [ ] **対象操作**: 作成/更新/削除/ログイン/権限変更
- [ ] **必須項目**: who(userId), when(timestamp), what(action), target(resourceId)
- [ ] **改ざん防止**: append-only + ハッシュチェーン
- [ ] **保持期間**: 7年（電帳法要件）
- [ ] **検索可能**: correlationId, tenantId, resourceIdでフィルタ

### 3.4 通信・データ保護
- [ ] **HTTPS強制**: 本番環境はHTTPSのみ
- [ ] **CORS**: 許可オリジン明示（`*` 禁止）
- [ ] **セキュリティヘッダー**: 
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Strict-Transport-Security`
- [ ] **機密情報**: ログ/レスポンスに含めない（APIキー、パスワード）
- [ ] **署名付きURL**: ファイルダウンロードは期限付きURL

---

## 4. テスト (Testing)

### 4.1 カバレッジ目標
- [ ] **ユニットテスト**: ビジネスロジック 80%以上
- [ ] **統合テスト**: 主要APIエンドポイント
- [ ] **E2Eテスト**: クリティカルパス（ログイン→主要機能→ログアウト）

### 4.2 テストケース（必須パターン）
- [ ] **正常系**: 有効入力 → 期待出力
- [ ] **異常系**: 無効入力 → 適切なエラー
- [ ] **境界値**: 最小値、最大値、空、null
- [ ] **権限**: 認証なし→401、権限なし→403
- [ ] **競合**: 同時更新、重複作成

```go
// ✅ Good: テーブル駆動テスト
func TestCreateInvoice(t *testing.T) {
    tests := []struct {
        name    string
        input   CreateInvoiceRequest
        wantErr error
    }{
        {"正常系", validRequest, nil},
        {"空の明細", emptyLinesRequest, ErrNoLines},
        {"不正な日付", invalidDateRequest, ErrValidation},
        {"権限なし", unauthorizedRequest, ErrForbidden},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

### 4.3 モック方針
- [ ] **外部API**: MSW / httptest でモック
- [ ] **DB**: インメモリ実装 or testcontainers
- [ ] **時間**: fake timer / clock interface
- [ ] **ファイル**: afero (Go) / memfs

### 4.4 アンチパターン回避
- [ ] **スナップショット依存**: 構造変更で大量失敗しない設計
- [ ] **テスト間依存**: 各テストは独立実行可能
- [ ] **フレーク対策**: リトライ、タイムアウト、並列実行考慮

---

## 5. 可搬性・抽象化 (Portability)

### 5.1 依存の抽象化
- [ ] **インターフェース定義**: 外部依存は全てインターフェース経由
- [ ] **DI**: コンストラクタインジェクション
- [ ] **実装差し替え**: Storage, Queue, Cache は実装切り替え可能

```go
// ✅ Good: インターフェース + DI
type Storage interface {
    Put(ctx context.Context, key string, data io.Reader) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
}

type Service struct {
    storage Storage  // S3でもR2でもMinIOでも差し替え可能
}

func NewService(storage Storage) *Service {
    return &Service{storage: storage}
}
```

### 5.2 環境非依存
- [ ] **パス**: 絶対パス禁止、環境変数 or 相対パス
- [ ] **OS依存**: ファイルパス区切りは `filepath.Join`
- [ ] **タイムゾーン**: UTCで保存、表示時に変換

### 5.3 Contract-First
- [ ] **OpenAPI優先**: 仕様変更 → OpenAPI → コード生成
- [ ] **生成コード不変**: 生成ファイルは手動編集禁止
- [ ] **ドリフト検出**: CI で `make lint-openapi` 実行

---

## チェック方法

```bash
# 1. OpenAPIドリフト検出
make lint-openapi

# 2. Go lint + test
cd apps/api && go vet ./... && go test -race ./...

# 3. Frontend lint + test
cd apps/web && pnpm lint && pnpm test

# 4. セキュリティヘッダー確認
curl -I http://localhost:8080/health
```

---

## 改善パッチの書き方

基準を満たしていない箇所があれば、PR本文に最小diffを添付：

```diff
// 例: タイムアウト追加
- client := &http.Client{}
+ client := &http.Client{
+     Timeout: cfg.HTTPClientTimeout,
+ }
```

```diff
// 例: 相関ID追加
- slog.Info("invoice created")
+ slog.Info("invoice created",
+     slog.String("correlationId", ctx.Value(CorrelationIDKey).(string)),
+     slog.String("tenantId", tenant.ID),
+ )
```

---

## クイックチェックリスト（PR前の最終確認）

- [ ] `make gen` 実行済み（OpenAPI→コード生成）
- [ ] `make lint-openapi` 通過（ドリフトなし）
- [ ] 全テスト通過
- [ ] 新規コードにテスト追加
- [ ] ログに correlationId/tenantId 含む
- [ ] エラー時のUI状態（Error state）実装
- [ ] 認可チェック実装（該当時）
- [ ] 監査ログ追加（変更系API）
