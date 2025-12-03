# 設計ギャップ分析ドキュメント v3
<!-- Last Updated: 2025-12-03 -->

## 目次
1. [現状サマリ](#現状サマリ)
2. [実装済みコンポーネント](#実装済みコンポーネント)
3. [不足設計リスト](#不足設計リスト)
4. [マネタイズ強化項目](#マネタイズ強化項目)
5. [優先度別ロードマップ](#優先度別ロードマップ)

---

## 現状サマリ

| コンポーネント | 状態 | 備考 |
|--------------|------|------|
| OpenAPI契約 | ✅ 完了 | audit-zip.yaml / jp-pint.yaml (3.0/3.1) |
| Goバックエンド | ⚠️ MVP | インメモリ実装、RBAC未実装 |
| Next.jsフロント | ⚠️ MVP | PWAオフラインキュー有、認証UI無 |
| CI/CD | ❌ 最小 | Markdown/JSON lint のみ |
| 本番インフラ | ❌ 未着手 | S3/DB/CDN/監視すべて未実装 |
| 認証/認可 | ❌ 未着手 | ヘッダ直書き（X-Tenant-Id） |
| 課金/テナント分離 | ❌ 未着手 | - |

---

## 実装済みコンポーネント

### ✅ API契約 (Contract-First)
- \`openapi/audit-zip.yaml\` (v3.1): Audit ZIP Export API
- \`openapi/jp-pint.yaml\` (v3.0): JP PINT Invoice API
- 型生成: \`oapi-codegen\` (Go) / \`openapi-typescript\` (TS)
- ロックファイル: \`.lock.yaml\` による差分検出

### ✅ バックエンド (Go + Chi)
- \`apps/api/internal/auditzip/\`: Audit ZIP ジョブ管理
  - インメモリQueue (\`queue.go\`)
  - インメモリStorage (\`storage.go\`)
  - Rate Limiter (\`rate_limiter.go\`)
  - 監査ログ ハッシュチェーン (\`audit.go\`)
  - Idempotency Key対応
- \`apps/api/internal/pint/\`: JP PINT 請求書
  - Validator (\`validator.go\`)
  - UBL XML生成 (\`ubl.go\`)
  - PDF生成（プレースホルダ） (\`pdf.go\`)
  - インメモリStorage

### ✅ フロントエンド (Next.js 15)
- \`apps/web/\`: PWA対応
  - Audit ZIP ウィザード UI
  - オフライン再送キュー (localStorage)
  - 環境変数による設定外出し

---

## 不足設計リスト

### 1. 認証/認可 [Critical - P0]
- OIDC プロバイダ選定 (Auth.js + Keycloak/Okta)
- JWTトークン検証ミドルウェア (Go)
- RBACテーブル設計 (roles, user_roles)
- テナント境界 enforcement
- CSRF/CSP/セキュアヘッダ

### 2. 永続化/マイグレーション [Critical - P0]
- PostgreSQL スキーマ設計
- Atlas/Goose マイグレーション
- Repository層実装 (SQLC)
- インメモリ→DB切替フラグ

### 3. オブジェクトストレージ [Critical - P0]
- S3/R2 クライアント実装
- 署名URL生成
- KMS暗号化
- テナント別プレフィックス強制
- ライフサイクルポリシー

### 4. ジョブ/非同期処理 [High - P1]
- Asynq (Redis) 導入
- リトライ・バックオフ
- デッドレターキュー
- キャンセル永続化

### 5. PDF生成 [High - P1]
- テンプレートエンジン
- 多言語対応
- フォント埋め込み

### 6. 可観測性 [High - P1]
- OpenTelemetry SDK
- Prometheusメトリクス
- アラートルール
- PIIマスキング

### 7. セキュリティ [Critical - P0]
- CSRF保護
- CSPヘッダ
- Rate Limit (Redis)
- ファイルスキャン

### 8. CI/CD [Critical - P0]
- 契約ドリフト検知 (oasdiff)
- Go/TS テスト
- E2E (Playwright)
- Docker ビルド
- デプロイパイプライン

### 9. テナント分離・課金 [High - P1]
- Stripe連携
- 使用量計測
- プラン別制限
- 月次バッチ

---

## マネタイズ強化項目

### 価格戦略

| プラン | 月額 | 請求書/月 | ストレージ |
|-------|------|----------|-----------|
| Free | ¥0 | 10通 | 1GB |
| Basic | ¥18,000 | 100通 | 10GB |
| Pro | ¥45,000 | 800通 | 100GB |
| Enterprise | 要相談 | 無制限 | 無制限 |

### 追加収益源
- 監査レポートパック: ¥50,000/回
- 導入支援: ¥300,000〜
- 法改正アップデート保守: ¥120,000/年

---

## 優先度別ロードマップ

### Phase 1: MVP安定化 (2週間)
- [ ] 認証/認可 P0
- [ ] CI/CD強化 P0
- [ ] セキュアヘッダ P0

### Phase 2: 本番化 (4週間)
- [ ] PostgreSQL移行 P0
- [ ] S3ストレージ P0
- [ ] Asynqジョブキュー P1
- [ ] 可観測性 P1

### Phase 3: 商用化 (4週間)
- [ ] テナント分離・課金 P1
- [ ] 法改正差分配信 P1
- [ ] 検索機能 P2
