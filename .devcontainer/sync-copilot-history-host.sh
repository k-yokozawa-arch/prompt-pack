#!/bin/bash
#=============================================================================
# Copilot Chat 履歴同期スクリプト（ホスト側で実行）
# initializeCommand で実行される
#
# DevContainer を開く前に、ホスト側の2つの workspace 間で履歴を同期する
#=============================================================================

set -euo pipefail

echo "�� [Host] Syncing Copilot chat history before container start..."

# ワークスペース ID
HOST_NATIVE_WS_ID="32985935220df26522ff317279b12fdd"
DEVCONTAINER_WS_ID="5917ff368333f296cd62a36f194a7c79"

# ホスト側の workspaceStorage パス（macOS）
HOST_WS_PATH="$HOME/Library/Application Support/Code/User/workspaceStorage"

NATIVE_PATH="$HOST_WS_PATH/$HOST_NATIVE_WS_ID"
DC_PATH="$HOST_WS_PATH/$DEVCONTAINER_WS_ID"

# ディレクトリ作成
mkdir -p "$DC_PATH/chatSessions" "$DC_PATH/chatEditingSessions"
mkdir -p "$NATIVE_PATH/chatSessions" "$NATIVE_PATH/chatEditingSessions"

# Native → DevContainer にコピー
echo "📋 Syncing Native → DevContainer..."
if [ -d "$NATIVE_PATH/chatSessions" ]; then
    find "$NATIVE_PATH/chatSessions" -maxdepth 1 -name "*.json" -type f 2>/dev/null | while read -r f; do
        [ -f "$f" ] || continue
        filename=$(basename "$f")
        src_size=$(stat -f%z "$f" 2>/dev/null || echo 0)
        dst_size=0
        [ -f "$DC_PATH/chatSessions/$filename" ] && dst_size=$(stat -f%z "$DC_PATH/chatSessions/$filename" 2>/dev/null || echo 0)
        if [ "$src_size" != "$dst_size" ] || [ ! -f "$DC_PATH/chatSessions/$filename" ]; then
            cp -f "$f" "$DC_PATH/chatSessions/"
            echo "   ✓ $filename"
        fi
    done
fi

# DevContainer → Native にコピー
echo "📋 Syncing DevContainer → Native..."
if [ -d "$DC_PATH/chatSessions" ]; then
    find "$DC_PATH/chatSessions" -maxdepth 1 -name "*.json" -type f 2>/dev/null | while read -r f; do
        [ -f "$f" ] || continue
        filename=$(basename "$f")
        src_size=$(stat -f%z "$f" 2>/dev/null || echo 0)
        dst_size=0
        [ -f "$NATIVE_PATH/chatSessions/$filename" ] && dst_size=$(stat -f%z "$NATIVE_PATH/chatSessions/$filename" 2>/dev/null || echo 0)
        if [ "$src_size" != "$dst_size" ] || [ ! -f "$NATIVE_PATH/chatSessions/$filename" ]; then
            cp -f "$f" "$NATIVE_PATH/chatSessions/"
            echo "   ✓ $filename"
        fi
    done
fi

# state.vscdb のインデックスをマージ（sqlite3 と jq が必要）
if command -v sqlite3 &>/dev/null && command -v jq &>/dev/null; then
    echo "🗄️  Merging session indices..."

    for src_db in "$NATIVE_PATH/state.vscdb" "$DC_PATH/state.vscdb"; do
        for dst_db in "$NATIVE_PATH/state.vscdb" "$DC_PATH/state.vscdb"; do
            [ "$src_db" = "$dst_db" ] && continue
            [ ! -f "$src_db" ] && continue

            src_index=$(sqlite3 "$src_db" "SELECT value FROM ItemTable WHERE key = 'chat.ChatSessionStore.index';" 2>/dev/null || echo "")
            [ -z "$src_index" ] && continue

            if [ ! -f "$dst_db" ]; then
                sqlite3 "$dst_db" "CREATE TABLE IF NOT EXISTS ItemTable (key TEXT PRIMARY KEY, value TEXT);"
            fi

            dst_index=$(sqlite3 "$dst_db" "SELECT value FROM ItemTable WHERE key = 'chat.ChatSessionStore.index';" 2>/dev/null || echo '{"version":1,"entries":{}}')
            merged=$(echo "$dst_index" "$src_index" | jq -s '{ version: 1, entries: ((.[0].entries // {}) + (.[1].entries // {})) }' 2>/dev/null || echo "$src_index")
            escaped=$(echo "$merged" | tr -d '\n' | sed "s/'/''/g")
            sqlite3 "$dst_db" "INSERT OR REPLACE INTO ItemTable (key, value) VALUES ('chat.ChatSessionStore.index', '$escaped');" 2>/dev/null || true
        done
    done
    echo "   ✓ Done"
else
    echo "   ⚠️ sqlite3 or jq not available, skipping index merge"
fi

echo "✅ [Host] Sync complete!"
