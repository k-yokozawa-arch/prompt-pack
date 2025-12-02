#!/usr/bin/env bash
set -euo pipefail

# VS Code サーバーを node 権限で確保（Codex のチャット履歴共有を想定）
bash -lc 'mkdir -p /home/node/.vscode-server && sudo chown -R node:node /home/node/.vscode-server || true'

# sqlite3 と jq をインストール（Copilot履歴同期で使用）
sudo apt-get update && sudo apt-get install -y sqlite3 jq

# Node パッケージマネージャ（pnpm v10 を system PATH に用意）
sudo corepack enable
sudo corepack prepare pnpm@10 --activate

# グローバル CLI（Playwright も含めてインストール）
npm i -g playwright @redocly/openapi-cli openapi-diff

# グローバル npm パス（playwright を require する際に必要）
PLAYWRIGHT_NODE_PATH=$(npm root -g)

# JS 側の依存
pnpm install

# Go ツール（OpenAPI 生成 & ホットリロード）
GOBIN=$(go env GOPATH)/bin GO111MODULE=on go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1
GOBIN=$(go env GOPATH)/bin go install github.com/air-verse/air@latest

# GOPATH/bin を PATH に追加
echo 'export PATH=$PATH:'"$(go env GOPATH)/bin" >> ~/.bashrc || true
echo 'export PATH=$PATH:'"$(go env GOPATH)/bin" >> ~/.zshrc || true

# Chromium（Playwright が配布するバイナリを利用）
sudo npx playwright install --with-deps chromium

# インストールされた Chromium の実体パスを取得して共有
PLAYWRIGHT_CHROMIUM_PATH=$(NODE_PATH="$PLAYWRIGHT_NODE_PATH" node -p "require('playwright').chromium.executablePath()")
export PDF_CHROMIUM_PATH="$PLAYWRIGHT_CHROMIUM_PATH"
echo "export PDF_CHROMIUM_PATH=$PLAYWRIGHT_CHROMIUM_PATH" >> ~/.bashrc || true
echo "export PDF_CHROMIUM_PATH=$PLAYWRIGHT_CHROMIUM_PATH" >> ~/.zshrc || true

# apps/api/.env は git 管理外の開発用ファイル。存在していれば更新し、無ければ作成する。
ENV_FILE="apps/api/.env"
if [ -f "$ENV_FILE" ]; then
  if grep -q '^PDF_CHROMIUM_PATH=' "$ENV_FILE"; then
    sed -i "s#^PDF_CHROMIUM_PATH=.*#PDF_CHROMIUM_PATH=$PLAYWRIGHT_CHROMIUM_PATH#" "$ENV_FILE"
  else
    printf '\nPDF_CHROMIUM_PATH=%s\n' "$PLAYWRIGHT_CHROMIUM_PATH" >> "$ENV_FILE"
  fi
else
  printf 'PDF_CHROMIUM_PATH=%s\n' "$PLAYWRIGHT_CHROMIUM_PATH" > "$ENV_FILE"
fi

# 生成スクリプトの初回実行
make gen || true

echo "✅ postCreate done"
