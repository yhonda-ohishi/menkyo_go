# インストールガイド

## 必要な環境

### 1. Go言語のインストール

1. https://golang.org/dl/ にアクセス
2. Windows用のインストーラーをダウンロード（Go 1.21以上）
3. インストーラーを実行
4. インストール後、コマンドプロンプトで確認:
   ```cmd
   go version
   ```

### 2. MinGW-w64のインストール（CGOに必須）

SQLiteライブラリ（go-sqlite3）を使用するため、C言語コンパイラが必要です。

#### オプション1: Chocolateyを使う（推奨）

```cmd
choco install mingw
```

#### オプション2: 手動インストール

1. https://www.mingw-w64.org/downloads/ にアクセス
2. 「MSYS2」または「MinGW-W64-builds」をダウンロード
3. インストール後、MinGWのbinディレクトリをPATHに追加
   - 例: `C:\mingw64\bin`

#### オプション3: MSYS2を使う

```cmd
# MSYS2をインストール後
pacman -S mingw-w64-x86_64-gcc
```

### 3. インストール確認

新しいコマンドプロンプトを開いて確認:

```cmd
gcc --version
```

以下のような出力が表示されればOK:
```
gcc.exe (MinGW-W64 x86_64-posix-seh, built by ...) 13.2.0
```

### 4. Protocol Buffersコンパイラのインストール

#### オプション1: Chocolateyを使う

```cmd
choco install protoc
```

#### オプション2: 手動インストール

1. https://github.com/protocolbuffers/protobuf/releases にアクセス
2. `protoc-<version>-win64.zip` をダウンロード
3. 解凍して`bin\protoc.exe`をPATHに追加

### 5. Goプラグインのインストール

```cmd
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

`%GOPATH%\bin`（通常は`%USERPROFILE%\go\bin`）がPATHに含まれていることを確認してください。

## プロジェクトのセットアップ

### 1. 依存関係のインストール

```cmd
cd C:\go\menkyo_go
go mod download
```

### 2. Protobufコードの生成

PowerShellの場合:
```powershell
.\generate_proto.bat
```

または:
```cmd
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/license.proto
```

### 3. ビルド

#### PowerShellを使う（推奨）

```powershell
.\build.ps1
```

#### バッチファイルを使う

新しいコマンドプロンプトを開いて（MinGWのPATHが有効な状態で）:

```cmd
build_with_cgo.bat
```

#### 手動ビルド

```cmd
set CGO_ENABLED=1
go build -o bin\server.exe .\cmd\server
go build -o bin\reader.exe .\cmd\reader
```

## トラブルシューティング

### エラー: "gcc: command not found"

**原因**: MinGW-w64がインストールされていないか、PATHに追加されていません。

**解決策**:
1. MinGW-w64をインストール
2. 環境変数PATHにMinGWのbinディレクトリを追加
3. **新しい**コマンドプロンプト/PowerShellを開く（既存のウィンドウではPATH変更が反映されません）

### エラー: "Binary was compiled with 'CGO_ENABLED=0'"

**原因**: CGOが無効な状態でビルドされています。

**解決策**:
```cmd
set CGO_ENABLED=1
go clean
go build -o bin\server.exe .\cmd\server
```

または`build_with_cgo.bat`を使用してください。

### エラー: "protoc: command not found"

**原因**: Protocol Buffersコンパイラがインストールされていません。

**解決策**:
```cmd
choco install protoc
```

または手動でダウンロードしてPATHに追加してください。

### NFCリーダーが認識されない

**原因**: NFCリーダーのドライバがインストールされていません。

**解決策**:
1. デバイスマネージャーを開く
2. NFCリーダーが表示されているか確認
3. ドライバが正しくインストールされているか確認
4. 必要に応じてメーカーのサイトから最新ドライバをダウンロード

## 環境変数の設定

`.env`ファイルをプロジェクトのルートディレクトリに作成:

```bash
# .envファイルの例（.env.exampleをコピーして編集）
SERVER_PORT=50051
SERVER_DB_PATH=license_server.db

GRPC_SERVER_ADDR=localhost:50051
READER_DB_PATH=license_reader.db
READER_ID=default
```

`.env.example`をコピーして使用することもできます:

```cmd
copy .env.example .env
```

その後、`.env`ファイルを編集して環境に合わせた値を設定してください。

## 次のステップ

インストールが完了したら、[USAGE.md](USAGE.md)を参照して使い方を確認してください。

## サポート

問題が解決しない場合は、以下を確認してください:

1. Goのバージョンが1.21以上か
2. GCCが正しくインストールされているか（`gcc --version`で確認）
3. 環境変数PATHが正しく設定されているか
4. 新しいコマンドプロンプト/PowerShellウィンドウで実行しているか
