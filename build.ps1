#!/usr/bin/env pwsh
# PowerShell build script for menkyo_go

Write-Host "Building menkyo_go applications..." -ForegroundColor Green

# MinGWのパスを追加
$mingwPaths = @(
    "C:\ProgramData\mingw64\mingw64\bin",
    "C:\msys64\mingw64\bin",
    "C:\mingw64\bin"
)

foreach ($path in $mingwPaths) {
    if (Test-Path $path) {
        $env:PATH = "$path;$env:PATH"
        Write-Host "Added MinGW path: $path" -ForegroundColor Cyan
        break
    }
}

# CGOを有効にする（go-sqlite3に必要）
$env:CGO_ENABLED = "1"

# 出力ディレクトリを作成
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

# Serverアプリをビルド
Write-Host "`nBuilding server..." -ForegroundColor Yellow
go build -o bin/server.exe ./cmd/server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to build server" -ForegroundColor Red
    exit 1
}
Write-Host "Server built successfully: bin/server.exe" -ForegroundColor Green

# Readerアプリをビルド（タイムスタンプ付き）
Write-Host "`nBuilding reader..." -ForegroundColor Yellow
$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$readerOutput = "bin/reader_$timestamp.exe"
$version = "1.0.0"
$ldflags = "-X main.Version=$version -X main.BuildTime=$timestamp"
go build -ldflags $ldflags -o $readerOutput ./cmd/reader
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to build reader" -ForegroundColor Red
    exit 1
}
Write-Host "Reader built successfully: $readerOutput" -ForegroundColor Green

# reader.exeへのコピーも作成（後方互換性）
Copy-Item $readerOutput "bin/reader.exe" -Force
Write-Host "Also copied to: bin/reader.exe" -ForegroundColor Cyan

Write-Host "`nBuild completed successfully!" -ForegroundColor Green
Write-Host "`nUsage:" -ForegroundColor Cyan
Write-Host "  Start server: bin\server.exe -port 50051 -db license_server.db"
Write-Host "  Start reader: bin\reader.exe -server localhost:50051 -db license_reader.db -reader-id reader01"
