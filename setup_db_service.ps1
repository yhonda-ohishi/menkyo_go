# db_serviceの必要な部分だけをsparse checkoutで取得するスクリプト

# クローン先のディレクトリ
$targetDir = "db_service"

# 既存のディレクトリがあれば削除
if (Test-Path $targetDir) {
    Write-Host "Removing existing $targetDir..."
    Remove-Item -Recurse -Force $targetDir
}

# sparse checkoutでクローン
Write-Host "Cloning db_service with sparse checkout..."
git clone --filter=blob:none --no-checkout https://github.com/yhonda-ohishi/db_service.git $targetDir

# ディレクトリに移動
Set-Location $targetDir

# sparse checkoutを設定
git sparse-checkout init --cone

# 必要なディレクトリだけを指定
Write-Host "Setting sparse checkout paths..."
git sparse-checkout set src/proto src/repository src/service cmd/server

# チェックアウト実行
Write-Host "Checking out files..."
git checkout main

Write-Host "Done! Only selected directories have been downloaded."
Write-Host ""
Write-Host "Downloaded directories:"
Get-ChildItem -Directory | Select-Object Name
