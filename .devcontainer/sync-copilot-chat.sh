#!/bin/bash
#=============================================================================
# Copilot Chat 履歴同期スクリプト (v3)
# 
# 機能:
#   - ホスト側の Native ↔ DevContainer workspace 間で履歴を同期
#   - ファイルサイズベースで比較（より大きいファイルを優先）
#   - セッションインデックスは lastMessageDate が新しい方を優先
#
# ⚠️  同期後、ホスト側の VS Code を再起動する必要があります！
#=============================================================================

set -euo pipefail

HOST_NATIVE_WS_ID="32985935220df26522ff317279b12fdd"
DEVCONTAINER_WS_ID="5917ff368333f296cd62a36f194a7c79"

if [ -d "/home/node/.host-workspaceStorage" ]; then
    HOST_WS_PATH="/home/node/.host-workspaceStorage"
else
    HOST_WS_PATH="$HOME/Library/Application Support/Code/User/workspaceStorage"
fi

DIRECTION="bidirectional"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --direction) DIRECTION="$2"; shift 2 ;;
        --dry-run) DRY_RUN=true; shift ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

log() { echo "$(date '+%H:%M:%S') $1"; }
log_success() { echo "$(date '+%H:%M:%S') ✅ $1"; }

# ファイルサイズを取得（Linux/macOS両対応）
get_file_size() {
    stat -c%s "$1" 2>/dev/null || stat -f%z "$1" 2>/dev/null || echo 0
}

# より大きいファイルを優先してコピー
sync_file() {
    local src="$1" dst="$2"
    local filename=$(basename "$src")
    
    [ ! -f "$src" ] && return 0
    
    local src_size=$(get_file_size "$src")
    local dst_size=0
    [ -f "$dst" ] && dst_size=$(get_file_size "$dst")
    
    # ソースの方が大きい場合のみコピー
    if [ ! -f "$dst" ] || [ "$src_size" -gt "$dst_size" ]; then
        $DRY_RUN && log "  [DRY-RUN] Would copy: $filename ($src_size > $dst_size)" && return 0
        cp -f "$src" "$dst"
        log "  ✓ $filename ($src_size bytes)"
    fi
}

sync_chat_sessions() {
    local src_dir="$1" dst_dir="$2" label="$3"
    
    log "📋 $label..."
    [ ! -d "$src_dir" ] && return 0
    
    mkdir -p "$dst_dir"
    shopt -s nullglob
    for f in "$src_dir"/*.json; do
        [ -f "$f" ] && sync_file "$f" "$dst_dir/$(basename "$f")"
    done
    shopt -u nullglob
}

# lastMessageDate が新しい方を優先してマージ
merge_session_index() {
    local src_db="$1" dst_db="$2" label="$3"
    
    log "🗄️  $label - session index..."
    
    command -v sqlite3 &>/dev/null || return 0
    command -v jq &>/dev/null || return 0
    [ ! -f "$src_db" ] && return 0
    
    local src_index dst_index merged escaped session_count
    src_index=$(sqlite3 "$src_db" "SELECT value FROM ItemTable WHERE key = 'chat.ChatSessionStore.index';" 2>/dev/null || echo "")
    [ -z "$src_index" ] && return 0
    
    if [ ! -f "$dst_db" ]; then
        $DRY_RUN && log "  [DRY-RUN] Would create state.vscdb" && return 0
        sqlite3 "$dst_db" "CREATE TABLE IF NOT EXISTS ItemTable (key TEXT PRIMARY KEY, value TEXT);"
    fi
    
    dst_index=$(sqlite3 "$dst_db" "SELECT value FROM ItemTable WHERE key = 'chat.ChatSessionStore.index';" 2>/dev/null || echo '{"version":1,"entries":{}}')
    
    # より新しい lastMessageDate を持つエントリを優先
    merged=$(echo "$dst_index" "$src_index" | jq -s '
    {
      version: 1,
      entries: (
        [.[0].entries // {}, .[1].entries // {}] | 
        map(to_entries) | 
        add | 
        group_by(.key) | 
        map(max_by(.value.lastMessageDate // 0)) | 
        from_entries
      )
    }' 2>/dev/null || echo "$src_index")
    
    $DRY_RUN && { session_count=$(echo "$merged" | jq '.entries | length' 2>/dev/null || echo "?"); log "  [DRY-RUN] Would merge ($session_count sessions)"; return 0; }
    
    escaped=$(echo "$merged" | tr -d '\n' | sed "s/'/''/g")
    sqlite3 "$dst_db" "INSERT OR REPLACE INTO ItemTable (key, value) VALUES ('chat.ChatSessionStore.index', '$escaped');"
    
    session_count=$(echo "$merged" | jq '.entries | length' 2>/dev/null || echo "?")
    log_success "Index merged ($session_count sessions)"
}

sync_workspace() {
    local src_path="$1" dst_path="$2" label="$3"
    
    log ""
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log "$label"
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    mkdir -p "$dst_path/chatSessions" "$dst_path/chatEditingSessions"
    sync_chat_sessions "$src_path/chatSessions" "$dst_path/chatSessions" "$label"
    merge_session_index "$src_path/state.vscdb" "$dst_path/state.vscdb" "$label"
}

main() {
    log "🔄 Copilot Chat History Sync (v3)"
    log "   Direction: $DIRECTION"
    $DRY_RUN && log "   Mode: DRY-RUN"
    
    local native_path="$HOST_WS_PATH/$HOST_NATIVE_WS_ID"
    local dc_path="$HOST_WS_PATH/$DEVCONTAINER_WS_ID"
    
    case "$DIRECTION" in
        native-to-dc)
            sync_workspace "$native_path" "$dc_path" "Native → DevContainer"
            ;;
        dc-to-native)
            sync_workspace "$dc_path" "$native_path" "DevContainer → Native"
            ;;
        bidirectional)
            sync_workspace "$dc_path" "$native_path" "DevContainer → Native"
            sync_workspace "$native_path" "$dc_path" "Native → DevContainer"
            ;;
    esac
    
    log ""
    log_success "Sync complete!"
    log ""
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log "⚠️  ホスト側の VS Code を再起動してください (Cmd+Q)"
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

main "$@"
