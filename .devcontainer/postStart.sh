#!/usr/bin/env bash
set -euo pipefail

echo "🚀 postStart: Ensuring all tools are installed..."

# Chromium の確認とインストール
check_chromium() {
  local chromium_path
  chromium_path=$(node -p "require('playwright').chromium.executablePath()" 2>/dev/null || echo "")
  if [ -n "$chromium_path" ] && [ -f "$chromium_path" ]; then
    echo "✅ Chromium already installed at: $chromium_path"
    return 0
  fi
  return 1
}

if ! check_chromium; then
  echo "📦 Installing Chromium via Playwright..."
  sudo npx playwright install-deps chromium
  npx playwright install chromium
  
  # パスを取得して環境変数を設定
  PLAYWRIGHT_NODE_PATH=$(npm root -g)
  PLAYWRIGHT_CHROMIUM_PATH=$(NODE_PATH="$PLAYWRIGHT_NODE_PATH" node -p "require('playwright').chromium.executablePath()" 2>/dev/null || echo "")
  
  if [ -n "$PLAYWRIGHT_CHROMIUM_PATH" ]; then
    export PDF_CHROMIUM_PATH="$PLAYWRIGHT_CHROMIUM_PATH"
    
    # bashrc/zshrc に追加（重複を避ける）
    grep -q "PDF_CHROMIUM_PATH" ~/.bashrc || echo "export PDF_CHROMIUM_PATH=$PLAYWRIGHT_CHROMIUM_PATH" >> ~/.bashrc
    grep -q "PDF_CHROMIUM_PATH" ~/.zshrc 2>/dev/null || echo "export PDF_CHROMIUM_PATH=$PLAYWRIGHT_CHROMIUM_PATH" >> ~/.zshrc 2>/dev/null || true
    
    # apps/api/.env に追加
    ENV_FILE="/workspaces/prompt-pack/apps/api/.env"
    if [ -f "$ENV_FILE" ]; then
      if grep -q '^PDF_CHROMIUM_PATH=' "$ENV_FILE"; then
        sed -i "s#^PDF_CHROMIUM_PATH=.*#PDF_CHROMIUM_PATH=$PLAYWRIGHT_CHROMIUM_PATH#" "$ENV_FILE"
      else
        printf '\nPDF_CHROMIUM_PATH=%s\n' "$PLAYWRIGHT_CHROMIUM_PATH" >> "$ENV_FILE"
      fi
    else
      printf 'PDF_CHROMIUM_PATH=%s\n' "$PLAYWRIGHT_CHROMIUM_PATH" > "$ENV_FILE"
    fi
    
    echo "✅ Chromium installed at: $PLAYWRIGHT_CHROMIUM_PATH"
  fi
fi

# air (Go hot reload) の確認
if ! command -v air &> /dev/null; then
  echo "📦 Installing air..."
  GOBIN=$(go env GOPATH)/bin go install github.com/air-verse/air@latest
else
  echo "✅ air already installed"
fi

# oapi-codegen の確認
if ! command -v oapi-codegen &> /dev/null; then
  echo "📦 Installing oapi-codegen..."
  GOBIN=$(go env GOPATH)/bin GO111MODULE=on go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1
else
  echo "✅ oapi-codegen already installed"
fi

# pnpm の確認
if ! command -v pnpm &> /dev/null; then
  echo "📦 Setting up pnpm..."
  sudo corepack enable
  sudo corepack prepare pnpm@10 --activate
else
  echo "✅ pnpm already installed"
fi

# node_modules の確認
if [ ! -d "/workspaces/prompt-pack/node_modules" ]; then
  echo "📦 Installing npm dependencies..."
  cd /workspaces/prompt-pack && pnpm install
else
  echo "✅ node_modules already present"
fi

# Copilot履歴同期（既存）
bash /workspaces/prompt-pack/.devcontainer/sync-to-host.sh || true

echo "✅ postStart complete"
