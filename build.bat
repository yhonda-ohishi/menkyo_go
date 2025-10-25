@echo off
echo Building menkyo_go applications...

REM CGOを有効にする（go-sqlite3に必要）
set CGO_ENABLED=1

REM 出力ディレクトリを作成
if not exist "bin" mkdir bin

REM Readerアプリをビルド
echo.
echo Building reader...
go build -o bin\reader.exe .\cmd\reader
if %ERRORLEVEL% NEQ 0 (
    echo Error: Failed to build reader
    exit /b 1
)
echo Reader built successfully: bin\reader.exe

REM Serverアプリをビルド
echo.
echo Building server...
go build -o bin\server.exe .\cmd\server
if %ERRORLEVEL% NEQ 0 (
    echo Error: Failed to build server
    exit /b 1
)
echo Server built successfully: bin\server.exe

echo.
echo Build completed successfully!
echo.
echo Usage:
echo   Start server: bin\server.exe -port 50051 -db license_server.db
echo   Start reader: bin\reader.exe -server localhost:50051 -db license_reader.db -reader-id reader01
