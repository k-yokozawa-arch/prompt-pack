# CI/CD設計書

## 現状
- prompt-ci.yml（Copilot用）のみ
- ビルド・デプロイパイプラインなし

## 目標アーキテクチャ

### GitHub Actions ワークフロー

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  lint-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      # Go
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: cd apps/api && go vet ./...
      - run: cd apps/api && go test -race -coverprofile=coverage.out ./...
      
      # Node.js
      - uses: pnpm/action-setup@v2
        with:
          version: 9
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'pnpm'
      - run: pnpm install --frozen-lockfile
      - run: pnpm lint
      - run: pnpm test

  openapi-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npx @redocly/cli lint openapi/*.yaml

  docker-build:
    runs-on: ubuntu-latest
    needs: [lint-and-test]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/build-push-action@v5
        with:
          context: ./apps/api
          push: false
          tags: prompt-pack-api:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

  deploy-staging:
    runs-on: ubuntu-latest
    needs: [docker-build]
    if: github.ref == 'refs/heads/main'
    environment: staging
    steps:
      - uses: actions/checkout@v4
      # Cloud Run / Fly.io デプロイ

  deploy-production:
    runs-on: ubuntu-latest
    needs: [deploy-staging]
    if: github.ref == 'refs/heads/main'
    environment: production
    steps:
      - uses: actions/checkout@v4
      # 本番デプロイ（手動承認後）
```

### Dockerfile

```dockerfile
# apps/api/Dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/audit-zip

FROM gcr.io/distroless/static-debian12
COPY --from=builder /server /server
EXPOSE 8080
CMD ["/server"]
```

## 環境管理
- Staging: 自動デプロイ（mainブランチ）
- Production: 手動承認後デプロイ
- Feature Preview: PR毎のプレビュー環境（オプション）

## シークレット管理
- GitHub Secrets for CI
- 本番: AWS Secrets Manager / GCP Secret Manager
