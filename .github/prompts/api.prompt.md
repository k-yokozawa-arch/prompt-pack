---
description: "OpenAPI 3.1 でAPI契約を定義"
---

# 📜 API設計（Contract-First）

## プロジェクトコンテキスト
- **API仕様**: [openapi/audit-zip.yaml](../../../openapi/audit-zip.yaml), [openapi/jp-pint.yaml](../../../openapi/jp-pint.yaml)
- **生成ツール**: oapi-codegen (Go), openapi-typescript (TS)
- **バージョン**: OpenAPI 3.1

## 設計原則

### 1. Contract-First
```
OpenAPI定義 → コード生成 → 実装
（逆は禁止）
```

### 2. エラーモデル（標準化）
```yaml
components:
  schemas:
    Error:
      type: object
      required: [code, message]
      properties:
        code:
          type: string
          enum: [validation_error, forbidden, conflict, internal_error]
        message:
          type: string
        details:
          type: array
          items:
            type: object
```

| Status | code | 用途 |
|--------|------|------|
| 400 | validation_error | 入力検証エラー |
| 403 | forbidden | 認可エラー |
| 409 | conflict | 競合（重複/同時実行） |
| 500 | internal_error | サーバーエラー |

### 3. 必須ヘッダー
```yaml
parameters:
  - name: X-Correlation-Id
    in: header
    required: true
    schema:
      type: string
  - name: X-Tenant-Id
    in: header
    required: true
    schema:
      type: string
```

### 4. 監査対応
- 変更系（POST/PUT/DELETE）は `auditId` を返す
- レスポンスに `createdAt`, `updatedAt` を含める

## 出力手順

1. [openapi/*.yaml](../../../openapi/) にエンドポイント追加
2. 型生成:
   ```bash
   make gen
   ```
3. ドリフト検出:
   ```bash
   make lint-openapi
   ```
4. ハンドラ実装

---

## 📝 API設計入力欄

**エンドポイント**: （例: POST /invoices/{id}/pdf）
**目的**: （このAPIが解決する課題）
**リクエスト**: （パラメータ/ボディ）
**レスポンス**: （成功時の出力）
**エラーケース**: （想定されるエラー）

---

上記に従ってOpenAPI定義を作成してください。
