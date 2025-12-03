# 可観測性設計書

## 現状
- 標準出力へのログ出力のみ
- メトリクス・トレーシングなし

## 目標アーキテクチャ

### 三本柱
1. **Logs** - 構造化ログ
2. **Metrics** - Prometheus形式メトリクス
3. **Traces** - OpenTelemetry分散トレーシング

## ログ設計

### 構造化ログ（slog）
```go
// internal/logging/logger.go
func NewLogger(env string) *slog.Logger {
    var handler slog.Handler
    if env == "production" {
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelInfo,
        })
    } else {
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        })
    }
    return slog.New(handler)
}

// 使用例
logger.Info("invoice created",
    slog.String("invoice_id", id),
    slog.String("tenant_id", tenantID),
    slog.String("correlation_id", correlationID),
)
```

### ログレベル
- ERROR: 即時対応が必要なエラー
- WARN: 注意が必要な状態
- INFO: 重要なビジネスイベント
- DEBUG: デバッグ情報（本番では無効）

## メトリクス設計

### Prometheusメトリクス
```go
var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )
    
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )
    
    invoicesCreated = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "invoices_created_total",
            Help: "Total invoices created",
        },
        []string{"tenant_id", "status"},
    )
)
```

### /metrics エンドポイント
```go
r.Handle("/metrics", promhttp.Handler())
```

## 分散トレーシング

### OpenTelemetry設定
```go
func initTracer() (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracehttp.New(context.Background())
    if err != nil {
        return nil, err
    }
    
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("prompt-pack-api"),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

## ヘルスチェック

```go
// GET /health
type HealthResponse struct {
    Status    string            `json:"status"`
    Version   string            `json:"version"`
    Checks    map[string]string `json:"checks"`
}

func (s *Service) HealthCheck(w http.ResponseWriter, r *http.Request) {
    checks := map[string]string{
        "database": s.checkDB(),
        "storage":  s.checkStorage(),
    }
    // ...
}
```

## アラート設定（Prometheus Alertmanager）
- エラー率 > 1%
- レイテンシ P99 > 1s
- ヘルスチェック失敗
