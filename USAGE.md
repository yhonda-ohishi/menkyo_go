# 使用方法

## 基本的な使い方

### 1. サーバーとリーダーを同じマシンで動かす場合

#### ステップ1: サーバーを起動

```cmd
bin\server.exe
```

デフォルト設定:
- ポート: 50051
- データベース: license_server.db

#### ステップ2: リーダーを起動

別のコマンドプロンプトで:

```cmd
bin\reader.exe
```

デフォルト設定:
- サーバー: localhost:50051
- データベース: license_reader.db
- リーダーID: default

#### ステップ3: 免許証をかざす

NFCリーダーに免許証をかざすと、自動的に読み取りが開始されます。

### 2. 複数のリーダーで運用する場合

#### サーバー（1台）

```cmd
bin\server.exe -port 50051 -db central_server.db
```

#### リーダー1

```cmd
bin\reader.exe -server server_ip:50051 -reader-id "entrance_reader" -db reader1.db
```

#### リーダー2

```cmd
bin\reader.exe -server server_ip:50051 -reader-id "exit_reader" -db reader2.db
```

## コマンドラインオプション

### server.exe

| オプション | デフォルト | 説明 |
|-----------|-----------|------|
| `-port` | 50051 | gRPCサーバーのポート番号 |
| `-db` | license_server.db | SQLiteデータベースファイルのパス |

例:
```cmd
bin\server.exe -port 9090 -db C:\data\server.db
```

### reader.exe

| オプション | デフォルト | 説明 |
|-----------|-----------|------|
| `-server` | localhost:50051 | gRPCサーバーのアドレス |
| `-db` | license_reader.db | SQLiteデータベースファイルのパス |
| `-reader-id` | default | リーダーの識別ID |

例:
```cmd
bin\reader.exe -server 192.168.1.100:50051 -reader-id "gate_A" -db C:\data\reader_a.db
```

## 読み取りデータの確認

### コンソール出力

リーダーアプリがカードを検出すると、以下のような情報が表示されます:

```
Card detected:
  Card ID: 3B888001000000009181C100D8A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6
  Card Type: driver_license
  ATR: 3b888001000000009181c100d8
  Expiry Date: a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
  Remain Count: 3
  FeliCa UID: 01234567890abcdef
Data pushed to server (Request ID: 123e4567-e89b-12d3-a456-426614174000)
```

### SQLiteデータベース

データベースを直接クエリすることもできます:

```cmd
sqlite3 license_reader.db
```

```sql
-- 最新の読み取り履歴を10件取得
SELECT * FROM read_history ORDER BY timestamp DESC LIMIT 10;

-- エラーログを確認
SELECT * FROM logs WHERE level = 'ERROR' ORDER BY timestamp DESC;

-- 特定のカードIDの履歴を確認
SELECT * FROM read_history WHERE card_id LIKE '%A1B2C3%';
```

## トラブルシューティング

### リーダーが見つからない

```
Error: No NFC readers found
```

**解決策:**
1. NFCリーダーがPCに接続されているか確認
2. デバイスマネージャーでドライバが正しくインストールされているか確認
3. 他のアプリケーション（NFCポートソフトウェアなど）がリーダーを使用していないか確認

### カードが読めない

```
Error reading card: SCardConnect failed: 0x8010000C
```

**解決策:**
1. 免許証がICカード対応（2013年以降発行）か確認
2. カードをリーダーにしっかりと密着させる
3. 読み取り回数が残っているか確認（残り回数0の場合は読み取り不可）
4. リーダーを再起動してみる

### gRPC接続エラー

```
Warning: Failed to connect to gRPC server: connection refused
```

**解決策:**
1. サーバーが起動しているか確認
2. サーバーのアドレスとポート番号が正しいか確認
3. ファイアウォールでポートがブロックされていないか確認
4. ネットワーク接続を確認（別マシンの場合）

注意: gRPCサーバーに接続できなくてもローカルログは動作します。

### データベースエラー

```
Failed to initialize database: unable to open database file
```

**解決策:**
1. データベースファイルのパスが正しいか確認
2. ディレクトリへの書き込み権限があるか確認
3. ディスクの空き容量があるか確認

## データのバックアップ

定期的にSQLiteデータベースをバックアップすることを推奨します:

```cmd
copy license_reader.db backup\reader_backup_20241025.db
copy license_server.db backup\server_backup_20241025.db
```

または、データをエクスポート:

```cmd
sqlite3 license_reader.db ".dump" > backup.sql
```

## セキュリティに関する注意

1. **データベースの保護**: データベースファイルには個人情報が含まれるため、適切なアクセス制御を設定してください

2. **通信の暗号化**: 本番環境では、gRPC通信にTLS/SSLを使用してください

3. **ログの管理**: ログファイルには個人情報が含まれる可能性があるため、適切に管理してください

4. **個人情報保護法対応**: 個人情報保護法に準拠した運用を行ってください

## 高度な使用方法

### データベースから統計情報を取得

```sql
-- 時間帯別の読み取り件数
SELECT
    strftime('%H', timestamp) as hour,
    COUNT(*) as count
FROM read_history
WHERE status = 'success'
GROUP BY hour
ORDER BY hour;

-- リーダー別の読み取り件数
SELECT
    reader_id,
    COUNT(*) as total_reads,
    SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as successful_reads,
    SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as failed_reads
FROM read_history
GROUP BY reader_id;

-- カード種別の分布
SELECT
    card_type,
    COUNT(*) as count
FROM read_history
WHERE status = 'success'
GROUP BY card_type;
```

### ログレベルでのフィルタリング

```sql
-- エラーとワーニングのみ表示
SELECT * FROM logs
WHERE level IN ('ERROR', 'WARNING')
ORDER BY timestamp DESC;
```
