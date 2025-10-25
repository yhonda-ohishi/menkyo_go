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

# Readerアプリをビルド
Write-Host "`nBuilding reader..." -ForegroundColor Yellow
go build -o bin/reader.exe ./cmd/reader
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to build reader" -ForegroundColor Red
    exit 1
}
Write-Host "Reader built successfully: bin/reader.exe" -ForegroundColor Green

Write-Host "`nBuild completed successfully!" -ForegroundColor Green
Write-Host "`nUsage:" -ForegroundColor Cyan
Write-Host "  Start server: bin\server.exe -port 50051 -db license_server.db"
Write-Host "  Start reader: bin\reader.exe -server localhost:50051 -db license_reader.db -reader-id reader01"
