# 免許証NFCリーダー (menkyo_go)

Windows環境で動作する日本の運転免許証のNFC読み取りシステムです。Go言語で実装され、gRPCを使って別プロセスにデータをプッシュし、SQLiteでログを記録します。

## 特徴

- **Windows PC/SC対応**: Windows標準のWinSCard APIを使用してNFCリーダーと通信
- **免許証自動判別**: ATR情報から免許証カードを自動検出
- **有効期限・残り回数取得**: 免許証の有効期限と残り読み取り回数を取得
- **gRPC通信**: 読み取ったデータを別プロセスにリアルタイムでプッシュ
- **SQLiteログ**: すべての読み取り履歴とログをSQLiteに永続化
- **車検証対応**: 免許証だけでなく車検証カードも検出可能

## システム構成

```
┌─────────────┐          gRPC          ┌─────────────┐
│   Reader    │ ────────────────────▶ │   Server    │
│  (reader)   │   License Data Push   │  (server)   │
└─────────────┘                        └─────────────┘
      │                                       │
      ▼                                       ▼
  [SQLite]                               [SQLite]
 reader.db                              server.db
```

- **Reader**: NFCリーダーでカードを読み取り、データをサーバーにプッシュ
- **Server**: gRPCサーバーとして動作し、データを受信・処理

## 必要要件

### ソフトウェア
- Windows 10/11
- Go 1.21以上
- Protocol Buffers compiler (protoc)
- NFCリーダードライバ（PC/SC互換）

### ハードウェア
- PC/SC対応のNFCリーダー
- 日本の運転免許証（ICカード）

## セットアップ

### 1. Goのインストール

https://golang.org/dl/ からGo 1.21以上をダウンロードしてインストール

### 2. Protocol Buffersのインストール

https://github.com/protocolbuffers/protobuf/releases から最新版をダウンロードして、`protoc.exe`をPATHに追加

### 3. 依存関係のインストール

```bash
go mod download
```

### 4. Protobufコードの生成

```bash
generate_proto.bat
```

## ビルド

### Readerアプリのビルド
```bash
go build -o bin/reader.exe ./cmd/reader
```

### Serverアプリのビルド
```bash
go build -o bin/server.exe ./cmd/server
```

### 一括ビルドスクリプト
```bash
build.bat
```

## 使い方

### 1. サーバーを起動

```bash
bin\server.exe -port 50051 -db license_server.db
```

オプション:
- `-port`: gRPCサーバーのポート番号（デフォルト: 50051）
- `-db`: SQLiteデータベースファイルのパス（デフォルト: license_server.db）

### 2. リーダーを起動

```bash
bin\reader.exe -server localhost:50051 -db license_reader.db -reader-id reader01
```

オプション:
- `-server`: gRPCサーバーのアドレス（デフォルト: localhost:50051）
- `-db`: SQLiteデータベースファイルのパス（デフォルト: license_reader.db）
- `-reader-id`: リーダーの識別ID（デフォルト: default）

### 3. 免許証をリーダーにかざす

リーダーアプリが免許証を検出すると、以下の情報が表示されます:
- カードID
- カード種別（driver_license / car_inspection / other）
- ATR（Answer To Reset）
- 有効期限（16進数）
- 残り読み取り回数
- FeliCa UID

データは自動的にサーバーにプッシュされ、両方のデータベースに記録されます。

## プロジェクト構造

```
menkyo_go/
├── cmd/
│   ├── reader/          # リーダーアプリケーション
│   │   └── main.go
│   └── server/          # サーバーアプリケーション
│       └── main.go
├── internal/
│   ├── nfc/             # NFC読み取り機能
│   │   ├── winscard.go      # Windows PC/SC API
│   │   └── license_reader.go # 免許証リーダーロジック
│   ├── database/        # SQLiteログ機能
│   │   └── logger.go
│   └── license/         # gRPC実装
│       ├── grpc_server.go
│       └── grpc_client.go
├── proto/               # gRPCプロトコル定義
│   └── license.proto
├── go.mod
├── generate_proto.bat   # Protobuf生成スクリプト
└── README.md
```

## 技術仕様

### Windows PC/SC API

`internal/nfc/winscard.go`では以下のWinSCard APIを使用:
- `SCardEstablishContext`: コンテキストの確立
- `SCardListReaders`: リーダーの列挙
- `SCardConnect`: カードへの接続
- `SCardTransmit`: APDUコマンドの送信
- `SCardGetStatusChange`: カード状態の監視
- `SCardDisconnect`: カードからの切断

### APDUコマンド

免許証読み取りに使用するAPDUコマンド（`internal/nfc/license_reader.go`）:

| コマンド | 説明 |
|---------|------|
| `CMD_START` | 初期化 |
| `CMD_START_TRANS` | トランザクション開始 |
| `CMD_CHECK_SHAKEN` | 車検証チェック |
| `CMD_SELECT_FELICA` | FeliCaカード選択 |
| `CMD_GET_FELICA_UID` | FeliCa UID取得 |
| `CMD_CHECK_REMAIN` | 残り回数照会 |
| `CMD_SELECT_EXPIRE_MF` | 有効期限MF選択 |
| `CMD_READ_EXPIRE_DF` | 有効期限DF読み取り |
| `CMD_SELECT_END` | 終了コマンド |

### データベーススキーマ

#### logsテーブル
```sql
CREATE TABLE logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    reader_id TEXT,
    card_id TEXT
)
```

#### read_historyテーブル
```sql
CREATE TABLE read_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    reader_id TEXT NOT NULL,
    card_id TEXT NOT NULL,
    card_type TEXT NOT NULL,
    atr TEXT,
    expiry_date TEXT,
    remain_count TEXT,
    felica_uid TEXT,
    status TEXT NOT NULL,
    error_message TEXT
)
```

## トラブルシューティング

### リーダーが見つからない

- NFCリーダーが正しく接続されているか確認
- デバイスマネージャーでドライバが正しくインストールされているか確認
- 他のアプリケーションがリーダーを使用していないか確認

### カードが読めない

- 免許証がICカード対応（2013年以降発行）か確認
- カードをリーダーにしっかりと密着させる
- 読み取り回数が残っているか確認（残り回数が0の場合は読み取り不可）

### gRPC接続エラー

- サーバーが起動しているか確認
- ファイアウォールでポートがブロックされていないか確認
- サーバーアドレスとポート番号が正しいか確認

## ライセンス

MIT License

## 参考資料

- Python実装: `printobserver.py`
- PC/SC Workgroup: https://www.pcscworkgroup.com/
- Protocol Buffers: https://protobuf.dev/
- gRPC: https://grpc.io/

## 注意事項

このシステムは個人情報を扱うため、以下の点に注意してください:
- データベースファイルは適切に保護してください
- 本番環境ではgRPC通信にTLS/SSL暗号化を使用してください
- 個人情報保護法に準拠した運用を行ってください
