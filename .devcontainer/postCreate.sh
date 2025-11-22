#!/usr/bin/env bash
set -euo pipefail

# Node パッケージマネージャ
corepack enable
corepack prepare pnpm@9 --activate

# JS 側の依存
pnpm install

# Go ツール（OpenAPI 生成 & ホットリロード）
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
go install github.com/cosmtrek/air@latest

# 生成スクリプトの初回実行
make gen || true

echo "✅ postCreate done"
