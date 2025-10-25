@echo off
setlocal

echo Building menkyo_go applications with CGO...
echo.

REM MinGWのパスを追加（一般的な場所を確認）
if exist "C:\ProgramData\mingw64\mingw64\bin\gcc.exe" (
    set "PATH=C:\ProgramData\mingw64\mingw64\bin;%PATH%"
    echo Added MinGW path: C:\ProgramData\mingw64\mingw64\bin
) else if exist "C:\msys64\mingw64\bin\gcc.exe" (
    set "PATH=C:\msys64\mingw64\bin;%PATH%"
    echo Added MinGW path: C:\msys64\mingw64\bin
) else if exist "C:\mingw64\bin\gcc.exe" (
    set "PATH=C:\mingw64\bin;%PATH%"
    echo Added MinGW path: C:\mingw64\bin
)

REM CGOを有効にする
set CGO_ENABLED=1

REM GCCの確認
where gcc >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Error: GCC not found in PATH
    echo Please install MinGW-w64 and add it to your PATH
    echo Download from: https://www.mingw-w64.org/downloads/
    exit /b 1
)

echo GCC found:
gcc --version | findstr "gcc"
echo.

REM 出力ディレクトリを作成
if not exist "bin" mkdir bin

REM Serverアプリをビルド
echo Building server...
go build -o bin\server.exe .\cmd\server
if %ERRORLEVEL% NEQ 0 (
    echo Error: Failed to build server
    exit /b 1
)
echo Server built successfully: bin\server.exe
echo.

REM Readerアプリをビルド
echo Building reader...
go build -o bin\reader.exe .\cmd\reader
if %ERRORLEVEL% NEQ 0 (
    echo Error: Failed to build reader
    exit /b 1
)
echo Reader built successfully: bin\reader.exe
echo.

echo Build completed successfully!
echo.
echo Usage:
echo   Start server: bin\server.exe -port 50051 -db license_server.db
echo   Start reader: bin\reader.exe -server localhost:50051 -db license_reader.db -reader-id reader01
echo.

endlocal
