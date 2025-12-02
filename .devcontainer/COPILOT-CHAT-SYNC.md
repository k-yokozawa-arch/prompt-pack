# Copilot Chat 履歴同期ガイド

## 概要

DevContainer とホスト側（macOS ネイティブ）で Copilot Chat 履歴を共有するための仕組みです。

## 同期の仕組み

### ワークスペース ID

同じプロジェクトでも、開き方によって異なるワークスペース ID が割り当てられます：

| ID | 説明 |
|----|------|
| `32985935220df26522ff317279b12fdd` | ホスト側でネイティブに開いた場合 |
| `5917ff368333f296cd62a36f194a7c79` | DevContainer で開いた場合 |

### 履歴ファイルの場所

| 環境 | パス |
|------|------|
| macOS ホスト | `~/Library/Application Support/Code/User/workspaceStorage/<workspace-id>/` |
| DevContainer | `/home/node/.host-workspaceStorage/<workspace-id>/`（マウント経由） |

### ファイル構造

```
workspaceStorage/<workspace-id>/
├── chatSessions/
│   └── <session-uuid>.json    # チャットセッションの内容
├── chatEditingSessions/       # 編集セッション
└── state.vscdb               # SQLite（セッションインデックス）
```

## 使い方

### DevContainer で作業する場合

```
┌─────────────────────────────────────────────────────────────┐
│  1. DevContainer を開く                                      │
│     └─ initializeCommand で自動同期                         │
│        （ホスト側の履歴 → DevContainer 側にコピー）          │
│                                                             │
│  2. 作業中                                                   │
│     └─ Copilot Chat はホスト側で動作（remote.extensionKind）│
│        履歴はホスト側の workspaceStorage に保存される        │
│                                                             │
│  3. 作業終了時（任意）                                       │
│     └─ 同期スクリプトを実行して履歴を双方向同期              │
└─────────────────────────────────────────────────────────────┘
```

#### 同期スクリプトの実行方法

**方法1: VS Code タスク（推奨）**
```
Cmd+Shift+P → "Tasks: Run Task" → "Sync Copilot Chat History (Bidirectional)"
```

**方法2: ターミナル**
```bash
.devcontainer/sync-copilot-chat.sh
```

### ホストネイティブで作業する場合

ホスト側のターミナルで実行：

```bash
cd ~/prompt-pack
.devcontainer/sync-copilot-chat.sh
```

※ 次回 DevContainer を開く際に `initializeCommand` で自動同期されるため、手動実行は必須ではありません。

## スクリプト一覧

| スクリプト | 用途 | 実行タイミング |
|-----------|------|----------------|
| `sync-copilot-history-host.sh` | Native→DC 初期同期 | `initializeCommand`（自動） |
| `sync-to-host.sh` | DC→Native 同期 | `postStartCommand`（自動） |
| `sync-copilot-chat.sh` | 双方向同期 | 手動 or VS Code タスク |
| `postCreate.sh` | 依存ツールのインストール | `postCreateCommand`（自動） |

### sync-copilot-chat.sh のオプション

```bash
# 双方向同期（デフォルト）
.devcontainer/sync-copilot-chat.sh

# 方向指定
.devcontainer/sync-copilot-chat.sh --direction native-to-dc
.devcontainer/sync-copilot-chat.sh --direction dc-to-native

# ドライラン（実際の変更なし）
.devcontainer/sync-copilot-chat.sh --dry-run
```

## 推奨ワークフロー

1. **DevContainer で作業開始** → 自動で同期される
2. **作業終了時** → 同期スクリプトを実行（任意）
3. **ホストネイティブで開く前** → VS Code を再起動（履歴をリロード）

## 同期ロジック

- **チャットセッション**: ファイルサイズが大きい方を優先（より多くの会話が含まれる）
- **セッションインデックス**: `lastMessageDate` が新しい方を優先

## トラブルシューティング

### 履歴が表示されない場合

1. **VS Code を再起動**（Cmd+Q → 再度開く）
   - 履歴はメモリにキャッシュされているため、再起動が必要
2. **同期スクリプトを実行**してファイルを同期

### 依存ツール

以下のツールが必要です（`postCreate.sh` でインストール済み）：

```bash
# 確認
which sqlite3 jq
```

## 技術的な制約

### リアルタイム同期が難しい理由

1. **VS Code のキャッシュ**: 履歴はメモリ上で管理され、ファイルへの書き込みは不定期
2. **ワークスペース ID の違い**: 同じプロジェクトでも開き方で異なる ID が割り当てられる
3. **SQLite のロック**: `state.vscdb` への同時アクセスは競合の可能性あり

### remote.extensionKind について

```json
"remote.extensionKind": {
  "github.copilot": ["ui"],
  "github.copilot-chat": ["ui"]
}
```

この設定により、Copilot Chat がホスト側で動作し、履歴がホスト側に保存されます。
DevContainer で作業中もホスト側に履歴が蓄積されるため、切り替え時の同期が容易になります。
