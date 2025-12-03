# ストレージ設計書

## 現状
- インメモリストレージ（InMemoryStorage）
- ファイル永続化なし

## 目標: S3互換オブジェクトストレージ

### 対応プロバイダー
- AWS S3
- Cloudflare R2（推奨：エグレス無料）
- MinIO（ローカル開発）

## インターフェース設計

```go
// internal/storage/storage.go
type ObjectStorage interface {
    Put(ctx context.Context, key string, data io.Reader, contentType string) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
    List(ctx context.Context, prefix string) ([]ObjectInfo, error)
}

type ObjectInfo struct {
    Key          string
    Size         int64
    LastModified time.Time
    ContentType  string
}
```

## S3実装

```go
// internal/storage/s3.go
type S3Storage struct {
    client *s3.Client
    bucket string
}

func NewS3Storage(cfg S3Config) (*S3Storage, error) {
    awsCfg, err := config.LoadDefaultConfig(context.Background(),
        config.WithRegion(cfg.Region),
    )
    if err != nil {
        return nil, err
    }
    
    client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        if cfg.Endpoint != "" {
            o.BaseEndpoint = aws.String(cfg.Endpoint)
            o.UsePathStyle = true // MinIO/R2互換
        }
    })
    
    return &S3Storage{
        client: client,
        bucket: cfg.Bucket,
    }, nil
}

func (s *S3Storage) Put(ctx context.Context, key string, data io.Reader, contentType string) error {
    _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      aws.String(s.bucket),
        Key:         aws.String(key),
        Body:        data,
        ContentType: aws.String(contentType),
    })
    return err
}
```

## パス設計（マルチテナント）

```
{bucket}/
├── {tenant_id}/
│   ├── invoices/
│   │   ├── {invoice_id}/
│   │   │   ├── invoice.xml      # UBL XML
│   │   │   ├── invoice.pdf      # PDF
│   │   │   └── metadata.json    # メタデータ
│   ├── exports/
│   │   ├── {job_id}.zip         # ZIPアーカイブ
│   └── audit/
│       └── {year}/{month}/
│           └── audit-log.jsonl  # 監査ログバックアップ
```

## 署名付きURL（プリサインドURL）

```go
func (s *S3Storage) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
    presignClient := s3.NewPresignClient(s.client)
    
    req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(s.bucket),
        Key:    aws.String(key),
    }, s3.WithPresignExpires(expiry))
    
    if err != nil {
        return "", err
    }
    return req.URL, nil
}
```

## 環境変数

```env
STORAGE_TYPE=s3           # memory | s3
S3_BUCKET=prompt-pack-storage
S3_REGION=auto            # Cloudflare R2
S3_ENDPOINT=              # カスタムエンドポイント（R2/MinIO）
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx
```

## ライフサイクルルール

- 一時ファイル: 24時間後削除
- エクスポートZIP: 30日後削除（ダウンロード済み）
- インボイス/監査ログ: 7年保持（電帳法）
