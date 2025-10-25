# ビルド成功！

## 確認済み環境

✅ **MinGW-w64**: `C:\ProgramData\mingw64\mingw64\bin\gcc.exe` (GCC 15.2.0)
✅ **CGO**: 有効
✅ **SQLite**: 正常動作
✅ **サーバー起動**: 成功（ポート50051）

## ビルド方法

### 方法1: PowerShellスクリプト（推奨）

```powershell
.\build.ps1
```

このスクリプトは自動的に以下を行います:
- MinGWのパスを検出して追加
- CGOを有効化
- serverとreaderをビルド

### 方法2: バッチファイル

```cmd
build_with_cgo.bat
```

### 方法3: 手動ビルド

```bash
# PATHにMinGWを追加
export PATH="/c/ProgramData/mingw64/mingw64/bin:$PATH"

# CGOを有効化
export CGO_ENABLED=1

# ビルド
cd /c/go/menkyo_go
go build -o bin/server.exe ./cmd/server
go build -o bin/reader.exe ./cmd/reader
```

## 起動方法

### サーバー起動

```cmd
bin\server.exe
```

または環境変数で設定:

```cmd
bin\server.exe -port 50051 -db license_server.db
```

### リーダー起動

```cmd
bin\reader.exe
```

または:

```cmd
bin\reader.exe -server localhost:50051 -db license_reader.db -reader-id reader01
```

## 環境変数（.env）

`.env.example`をコピーして`.env`として保存:

```bash
cp .env.example .env
```

`.env`の例:
```
SERVER_PORT=50051
SERVER_DB_PATH=license_server.db
GRPC_SERVER_ADDR=localhost:50051
READER_DB_PATH=license_reader.db
READER_ID=default
```

## トラブルシューティング

### ポートが既に使用されている

```
Failed to listen: bind: Only one usage of each socket address
```

**解決策:**

1. 使用中のプロセスを確認:
   ```cmd
   netstat -ano | findstr :50051
   ```

2. プロセスを終了:
   ```cmd
   taskkill /PID <PID番号> /F
   ```

3. または別のポートを使用:
   ```cmd
   bin\server.exe -port 50052
   ```

### CGOエラー

```
Binary was compiled with 'CGO_ENABLED=0'
```

**解決策:**

MinGWが正しくインストールされ、PATHに追加されていることを確認:

```cmd
where gcc
gcc --version
```

`build_with_cgo.bat`または`build.ps1`を使用してビルド。

## データベース確認

サーバー起動後、SQLiteデータベースが作成されます:

```cmd
sqlite3 license_server.db
```

```sql
-- テーブル確認
.tables

-- ログ確認
SELECT * FROM logs ORDER BY timestamp DESC LIMIT 10;

-- 読み取り履歴確認
SELECT * FROM read_history ORDER BY timestamp DESC LIMIT 10;
```

## 次のステップ

1. サーバーを起動
2. リーダーを起動
3. NFCリーダーに免許証をかざす
4. データベースでログを確認

詳細は[USAGE.md](USAGE.md)を参照してください。
