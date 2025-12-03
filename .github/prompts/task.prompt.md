---
description: "仕様→設計→実装→テスト→PRまで一括実行"
---

# 🎯 タスク実行プロンプト

## プロジェクトコンテキスト
- **アプリ**: 電帳法アーカイブ＋JP PINTインボイス（中小企業向けB2B SaaS）
- **バックエンド**: Go + Chi + oapi-codegen
- **フロントエンド**: Next.js 15 + TypeScript + PWA
- **API契約**: [openapi/*.yaml](../../../openapi/)（Contract-First）
- **品質基準**: [rubric.md](../../../rubric.md)

## 実装制約（必須遵守）

### Contract-First
- OpenAPI 3.1 を**先に**定義 → コード生成
- 生成コマンド: `make gen`
- ドリフト検出: `make lint-openapi`

### 設定管理
- ハードコード禁止
- 全設定は環境変数から注入
- 参照: [apps/api/internal/*/config.go](../../../apps/api/internal/)

### ログ・監査
```go
// 必須フィールド
slog.Info("action",
    slog.String("correlationId", corrId),
    slog.String("tenantId", tenantId),
    slog.String("action", "invoice.created"),
)
```
- 監査ログ: append-only + ハッシュチェーン（電帳法7年保持）

### UI状態（4状態必須）
- ⏳ Loading（スケルトン/スピナー）
- 📭 Empty（データなし時のCTA）
- ❌ Error（リトライ可能なUI）
- ✅ Success（トースト/完了表示）

### アクセシビリティ
- セマンティックHTML（`<main>`, `<nav>`, `<article>`）
- `aria-label`, `aria-describedby`
- フォーカス管理（モーダル/フォーム）
- コントラスト比 4.5:1 以上

## 実行フロー

### 1️⃣ 仕様確認
- 不明点は**仮定を明記**して進行
- 既存パターンを調査

### 2️⃣ API契約
```bash
# 1. openapi/*.yaml を編集
# 2. 型生成
make gen
# 3. ドリフト検出
make lint-openapi
```

### 3️⃣ バックエンド
- [apps/api/internal/{domain}/](../../../apps/api/internal/) にハンドラ追加
- 既存パターンに従う（`handler.go`, `service.go`）

### 4️⃣ フロントエンド
- [apps/web/src/app/](../../../apps/web/src/app/) にページ追加
- API呼び出し: [apps/web/src/lib/api/](../../../apps/web/src/lib/api/)

### 5️⃣ テスト
- 正常系 / 異常系 / 境界値
- モック: 外部依存は全てモック

### 6️⃣ 自己レビュー
- [rubric.md](../../../rubric.md) のチェックリストを通過させる

### 7️⃣ PR作成
- 変更概要 / リスク / ロールバック手順

---

## 📝 タスク入力欄

以下の形式で指示してください：

**何を**: （実装する機能）
**誰のために**: （対象ユーザー）
**入力**: （API/UIへの入力）
**出力**: （期待する結果）
**成功指標**: （KPI/SLO）
**補足**: （制約/優先度/参考情報）

---

上記に従って実装を開始してください。
