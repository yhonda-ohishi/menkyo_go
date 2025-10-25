@echo off
echo Generating protobuf code...

REM protocがインストールされているか確認
where protoc >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Error: protoc not found. Please install Protocol Buffers compiler.
    echo Download from: https://github.com/protocolbuffers/protobuf/releases
    exit /b 1
)

REM go-grpc pluginがインストールされているか確認
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

REM 出力ディレクトリを作成
if not exist "proto\license" mkdir proto\license

REM protoファイルをコンパイル
protoc --go_out=. --go_opt=paths=source_relative ^
    --go-grpc_out=. --go-grpc_opt=paths=source_relative ^
    proto/license.proto

if %ERRORLEVEL% EQU 0 (
    echo Protobuf code generated successfully!
) else (
    echo Error: Failed to generate protobuf code.
    exit /b 1
)
