# Git submoduleでdb_serviceを追加し、sparse checkoutで一部だけ取得

$submodulePath = "db_service"

Write-Host "Adding db_service as submodule with sparse checkout..."

# submoduleを追加（まだチェックアウトしない）
git submodule add --no-checkout https://github.com/yhonda-ohishi/db_service.git $submodulePath

# submoduleディレクトリに移動
Set-Location $submodulePath

# sparse checkoutを設定
git sparse-checkout init --cone
git sparse-checkout set src/proto src/repository cmd/server

# チェックアウト実行
git checkout main

# 元のディレクトリに戻る
Set-Location ..

Write-Host ""
Write-Host "Submodule added with sparse checkout!"
Write-Host "Only the following directories were downloaded:"
Write-Host "  - src/proto"
Write-Host "  - src/repository"
Write-Host "  - cmd/server"
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Rename go.work.template to go.work"
Write-Host "  2. Run: go mod tidy"
