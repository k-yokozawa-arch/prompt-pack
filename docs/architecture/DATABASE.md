# データベース設計書

## 現状
- インメモリストレージ（map + sync.RWMutex）
- 永続化なし

## 目標: PostgreSQL + Row-Level Security

### スキーマ設計

```sql
-- テナント管理
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    plan VARCHAR(50) NOT NULL DEFAULT 'free',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- API Keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id),
    key_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    scopes TEXT[],
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- インボイス
CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id),
    status VARCHAR(50) NOT NULL,
    supplier JSONB NOT NULL,
    customer JSONB NOT NULL,
    lines JSONB NOT NULL,
    issue_date DATE NOT NULL,
    due_date DATE,
    currency VARCHAR(3) NOT NULL,
    ubl_xml TEXT,
    pdf_url VARCHAR(500),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 監査ログ（電帳法対応）
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id),
    record_type VARCHAR(50) NOT NULL,
    record_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    hash VARCHAR(64) NOT NULL,
    prev_hash VARCHAR(64),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ZIP エクスポートジョブ
CREATE TABLE export_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id),
    status VARCHAR(50) NOT NULL,
    from_date DATE NOT NULL,
    to_date DATE NOT NULL,
    result_url VARCHAR(500),
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Row-Level Security
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON invoices
    USING (tenant_id = current_setting('app.tenant_id')::UUID);
```

### マイグレーション戦略
- golang-migrate または Atlas を使用
- CI/CDでマイグレーション自動実行
- ロールバック対応必須

### インデックス設計
```sql
CREATE INDEX idx_invoices_tenant_created ON invoices(tenant_id, created_at DESC);
CREATE INDEX idx_invoices_status ON invoices(status) WHERE status != 'completed';
CREATE INDEX idx_audit_logs_record ON audit_logs(record_type, record_id);
CREATE INDEX idx_export_jobs_tenant_status ON export_jobs(tenant_id, status);
```

## 接続管理
- コネクションプール: pgxpool
- 最大接続数: 環境変数で設定可能
- ヘルスチェック: /health エンドポイント
