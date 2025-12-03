# 課金・マネタイズ設計書

## ターゲット市場
- 中小企業（SMB）向けB2Bアプリ
- 電帳法アーカイブ + インボイス（JP PINT）

## 料金プラン

### Free（無料）
- 月10件までのインボイス発行
- PDF出力なし（UBL XMLのみ）
- ZIPエクスポートなし
- コミュニティサポート

### Basic（¥980/月）
- 月100件までのインボイス発行
- PDF出力対応
- 月1回のZIPエクスポート
- メールサポート

### Pro（¥4,980/月）
- 月1,000件までのインボイス発行
- PDF出力対応（カスタムテンプレート）
- 無制限ZIPエクスポート
- API優先アクセス
- 優先サポート

### Enterprise（要問合せ）
- 無制限インボイス
- SLA保証（99.9%）
- 専用サポート
- カスタム連携
- オンプレミス対応相談

## アドオンサービス

| サービス | 価格 | 対象プラン |
|---------|------|-----------|
| 追加インボイス（100件） | ¥500 | Basic以上 |
| カスタムPDFテンプレート | ¥10,000/初期 | Pro以上 |
| API連携支援 | ¥50,000〜 | Pro以上 |
| 税理士連携機能 | ¥2,000/月 | Basic以上 |

## 技術実装

### 使用量トラッキング
```go
type UsageTracker interface {
    IncrementInvoiceCount(ctx context.Context, tenantID string) error
    GetMonthlyUsage(ctx context.Context, tenantID string) (*Usage, error)
    CheckLimit(ctx context.Context, tenantID string, resource string) (bool, error)
}
```

### 課金連携
- Stripe Billing推奨
- 請求書払い対応（Enterprise）
- 日本円決済対応

### 制限適用
```go
func (s *Service) CreateInvoice(ctx context.Context, req CreateInvoiceRequest) error {
    tenant := GetTenant(ctx)
    
    // 使用量チェック
    allowed, err := s.usage.CheckLimit(ctx, tenant.ID, "invoices")
    if err != nil {
        return err
    }
    if !allowed {
        return ErrQuotaExceeded
    }
    
    // インボイス作成処理...
    
    // 使用量カウント
    s.usage.IncrementInvoiceCount(ctx, tenant.ID)
    return nil
}
```

## KPI
- MRR（月次経常収益）
- Churn Rate（解約率）
- LTV（顧客生涯価値）
- CAC（顧客獲得コスト）
