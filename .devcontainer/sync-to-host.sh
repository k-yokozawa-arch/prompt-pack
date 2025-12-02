#!/bin/bash
# Dev Container側からホスト側へCopilot履歴を同期するスクリプト
# Dev Container内で実行される

set -euo pipefail

echo "🔄 [Container→Host] Syncing Copilot chat history to host..."

# Dev Container内のworkspaceStorage
DC_WS_PATH="/home/node/.vscode-server/data/User/workspaceStorage"

# ホスト側のworkspaceStorage（マウントされている）
HOST_WS_PATH="/home/node/.host-workspaceStorage"

# Dev Container用のworkspace ID
DC_WS="5917ff368333f296cd62a36f194a7c79"

# ホスト側ネイティブのworkspace ID
HOST_NATIVE_WS="32985935220df26522ff317279b12fdd"

# マウントが存在するか確認
if [ ! -d "$HOST_WS_PATH" ]; then
    echo "⚠️  Host workspaceStorage not mounted at $HOST_WS_PATH"
    exit 1
fi

# コピー先ディレクトリを作成
mkdir -p "$HOST_WS_PATH/$HOST_NATIVE_WS/chatSessions"
mkdir -p "$HOST_WS_PATH/$HOST_NATIVE_WS/chatEditingSessions"
mkdir -p "$HOST_WS_PATH/$DC_WS/chatSessions"
mkdir -p "$HOST_WS_PATH/$DC_WS/chatEditingSessions"

# Dev Container内のchatSessionsをホスト側にコピー
echo "📋 Syncing chatSessions to host..."
if [ -d "$DC_WS_PATH/$DC_WS/chatSessions" ]; then
    shopt -s nullglob
    for f in "$DC_WS_PATH/$DC_WS/chatSessions/"*.json; do
        if [ -f "$f" ]; then
            filename=$(basename "$f")
            # 両方のworkspaceにコピー
            for target_ws in "$HOST_NATIVE_WS" "$DC_WS"; do
                target="$HOST_WS_PATH/$target_ws/chatSessions/$filename"
                if [ ! -f "$target" ] || [ "$f" -nt "$target" ]; then
                    cp "$f" "$target"
                    echo "   ✓ Copied to $target_ws: $filename"
                fi
            done
        fi
    done
    shopt -u nullglob
else
    echo "   ⚠️ No chatSessions found in container"
fi

# Dev Container内のchatEditingSessionsをホスト側にコピー
echo "📝 Syncing chatEditingSessions to host..."
if [ -d "$DC_WS_PATH/$DC_WS/chatEditingSessions" ]; then
    shopt -s nullglob
    for dir in "$DC_WS_PATH/$DC_WS/chatEditingSessions/"*/; do
        if [ -d "$dir" ]; then
            session_id=$(basename "$dir")
            for target_ws in "$HOST_NATIVE_WS" "$DC_WS"; do
                target_dir="$HOST_WS_PATH/$target_ws/chatEditingSessions/$session_id"
                if [ ! -d "$target_dir" ]; then
                    mkdir -p "$target_dir"
                    cp -r "$dir"* "$target_dir/" 2>/dev/null || true
                    echo "   ✓ Copied to $target_ws: $session_id"
                fi
            done
        fi
    done
    shopt -u nullglob
fi

# state.vscdbのインデックスをマージ
echo "🗄️  Updating session index..."
if command -v sqlite3 &> /dev/null && command -v jq &> /dev/null; then
    DC_STATE_DB="$DC_WS_PATH/$DC_WS/state.vscdb"

    if [ -f "$DC_STATE_DB" ]; then
        DC_INDEX=$(sqlite3 "$DC_STATE_DB" "SELECT value FROM ItemTable WHERE key = 'chat.ChatSessionStore.index';" 2>/dev/null || echo "")

        if [ -n "$DC_INDEX" ]; then
            for target_ws in "$HOST_NATIVE_WS" "$DC_WS"; do
                HOST_STATE_DB="$HOST_WS_PATH/$target_ws/state.vscdb"

                if [ -f "$HOST_STATE_DB" ]; then
                    HOST_INDEX=$(sqlite3 "$HOST_STATE_DB" "SELECT value FROM ItemTable WHERE key = 'chat.ChatSessionStore.index';" 2>/dev/null || echo '{"version":1,"entries":{}}')

                    MERGED=$(echo "$HOST_INDEX" "$DC_INDEX" | jq -s '{version:1,entries:((.[0].entries//{}) + (.[1].entries//{}))}' 2>/dev/null || echo "$DC_INDEX")

                    sqlite3 "$HOST_STATE_DB" "INSERT OR REPLACE INTO ItemTable (key, value) VALUES ('chat.ChatSessionStore.index', '$(echo "$MERGED" | tr -d '\n' | sed "s/'/''/g")');" 2>/dev/null || true
                    echo "   ✓ Merged index to $target_ws"
                fi
            done
        fi
    fi
else
    echo "   ⚠️ sqlite3 or jq not available"
fi

echo "✅ [Container→Host] Sync complete!"
