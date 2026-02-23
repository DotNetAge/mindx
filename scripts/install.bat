@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

:: MindX Windows Installation Script

echo ========================================
echo   MindX Windows Installation Script
echo ========================================
echo.

:: Get script directory
set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"
cd /d "%SCRIPT_DIR%"

:: Check if running from source or release
if exist "cmd\main.go" (
    set "INSTALL_MODE=source"
    echo Installation mode: Source
) else (
    set "INSTALL_MODE=release"
    echo Installation mode: Release package
)

:: Read version
if exist "VERSION" (
    set /p VERSION=<VERSION
) else (
    set "VERSION=latest"
)
echo Version: %VERSION%
echo.

:: Check prerequisites
echo [1/7] Checking prerequisites...
echo.

:: Check Ollama
where ollama >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Ollama is installed
    set "OLLAMA_AVAILABLE=true"
) else (
    echo [WARN] Ollama is not installed, installing now...
    echo.
    echo Installing Ollama for Windows...
    
    :: Use PowerShell to install Ollama
    powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://ollama.com/install.ps1 | iex"
    
    :: Verify installation
    where ollama >nul 2>&1
    if %errorlevel% equ 0 (
        echo [OK] Ollama installed successfully
        set "OLLAMA_AVAILABLE=true"
    ) else (
        echo [ERROR] Ollama installation failed
        echo.
        echo Please install Ollama manually from: https://ollama.com/download
        pause
        exit /b 1
    )
)

echo.

:: Set installation paths
echo [2/7] Setting up paths...
echo.

set "MINDX_PATH=%LOCALAPPDATA%\MindX"
set "MINDX_WORKSPACE=%USERPROFILE%\.mindx"

echo Install path: %MINDX_PATH%
echo Workspace: %MINDX_WORKSPACE%
echo.

:: Prepare binary
echo [3/7] Preparing binary...
echo.

if "%INSTALL_MODE%"=="source" (
    echo Building from source is not supported in Windows batch script.
    echo Please use the pre-built release package.
    pause
    exit /b 1
) else (
    if exist "bin\mindx.exe" (
        echo [OK] Found mindx.exe in bin\
    ) else if exist "mindx.exe" (
        if not exist "bin" mkdir bin
        copy mindx.exe bin\ >nul
        echo [OK] Copied mindx.exe to bin\
    ) else (
        echo [ERROR] mindx.exe not found
        pause
        exit /b 1
    )
)

echo.

:: Install to MINDX_PATH
echo [4/7] Installing files to %MINDX_PATH%...
echo.

if not exist "%MINDX_PATH%" mkdir "%MINDX_PATH%"
if not exist "%MINDX_PATH%\bin" mkdir "%MINDX_PATH%\bin"

:: Copy binary
copy /y "bin\mindx.exe" "%MINDX_PATH%\bin\" >nul
echo [OK] Copied mindx.exe

:: Copy skills
if exist "skills" (
    if not exist "%MINDX_PATH%\skills" mkdir "%MINDX_PATH%\skills"
    xcopy /y /e /q "skills\*" "%MINDX_PATH%\skills\" >nul 2>&1
    echo [OK] Copied skills
)

:: Copy static files
if exist "static" (
    if not exist "%MINDX_PATH%\static" mkdir "%MINDX_PATH%\static"
    xcopy /y /e /q "static\*" "%MINDX_PATH%\static\" >nul 2>&1
    echo [OK] Copied static files
)

:: Copy config templates
if exist "config" (
    if not exist "%MINDX_PATH%\config" mkdir "%MINDX_PATH%\config"
    for %%f in (config\*) do (
        if exist "%%f" (
            set "filename=%%~nxf"
            copy /y "%%f" "%MINDX_PATH%\config\!filename!.template" >nul 2>&1
        )
    )
    echo [OK] Copied config templates
)

:: Copy uninstall script
if exist "uninstall.bat" (
    copy /y "uninstall.bat" "%MINDX_PATH%\" >nul
    echo [OK] Copied uninstall.bat
)

echo.

:: Create workspace
echo [5/7] Creating workspace directory...
echo.

if not exist "%MINDX_WORKSPACE%" mkdir "%MINDX_WORKSPACE%"
if not exist "%MINDX_WORKSPACE%\config" mkdir "%MINDX_WORKSPACE%\config"
if not exist "%MINDX_WORKSPACE%\logs" mkdir "%MINDX_WORKSPACE%\logs"
if not exist "%MINDX_WORKSPACE%\data" mkdir "%MINDX_WORKSPACE%\data"
if not exist "%MINDX_WORKSPACE%\data\memory" mkdir "%MINDX_WORKSPACE%\data\memory"
if not exist "%MINDX_WORKSPACE%\data\sessions" mkdir "%MINDX_WORKSPACE%\data\sessions"
if not exist "%MINDX_WORKSPACE%\data\training" mkdir "%MINDX_WORKSPACE%\data\training"
if not exist "%MINDX_WORKSPACE%\data\vectors" mkdir "%MINDX_WORKSPACE%\data\vectors"

echo [OK] Created workspace: %MINDX_WORKSPACE%
echo.

:: Setup configuration
echo [6/7] Setting up configuration...
echo.

if exist "%MINDX_PATH%\config" (
    for %%t in ("%MINDX_PATH%\config\*.template") do (
        if exist "%%t" (
            set "template=%%t"
            set "filename=%%~nt"
            set "dest=%MINDX_WORKSPACE%\config\!filename!"
            if not exist "!dest!" (
                copy /y "%%t" "!dest!" >nul 2>&1
                echo [OK] Created config: !filename!
            ) else (
                echo [INFO] Config exists: !filename!
            )
        )
    )
)

echo.

:: Setup .env file
echo [7/7] Setting up environment...
echo.

:: Create .env file in workspace if not exists
if not exist "%MINDX_WORKSPACE%\.env" (
    (
        echo # MindX Environment Configuration
        echo MINDX_PATH=%MINDX_PATH%
        echo MINDX_WORKSPACE=%MINDX_WORKSPACE%
    ) > "%MINDX_WORKSPACE%\.env"
    echo [OK] Created .env file in workspace
) else (
    echo [INFO] .env file exists in workspace
)

echo.

:: Pull Ollama models
echo [8/8] Pulling Ollama models...
echo.

:: Models to pull (same as Linux/macOS)
set "MODELS=qllama/bge-small-zh-v1.5:latest qwen3:1.7b qwen3:0.6b"

for %%m in (%MODELS%) do (
    echo.
    echo Checking %%m...
    
    :: Check if model is already installed
    ollama list 2>nul | findstr /c:"%%m" >nul
    if !errorlevel! equ 0 (
        echo [OK] %%m is already installed
    ) else (
        echo Pulling %%m...
        ollama pull %%m
        if !errorlevel! equ 0 (
            echo [OK] Pulled %%m
        ) else (
            echo [WARN] Failed to pull %%m (will try later)
        )
    )
)

echo.

:: Print summary
echo ========================================
echo   Installation Complete!
echo ========================================
echo.
echo MindX has been successfully installed!
echo.
echo Install path: %MINDX_PATH%
echo Workspace:    %MINDX_WORKSPACE%
echo Binary:       %MINDX_PATH%\bin\mindx.exe
echo.
echo [IMPORTANT] Add MindX to your PATH:
echo.
echo 1. Open System Properties (Win + R, type sysdm.cpl)
echo 2. Go to Advanced tab
echo 3. Click Environment Variables
echo 4. Under User variables, find "Path"
echo 5. Click Edit and add: %MINDX_PATH%\bin
echo.
echo Or run this command in PowerShell (as Administrator):
echo.
echo [Environment]::SetEnvironmentVariable("Path", $env:Path + ";%MINDX_PATH%\bin", "User")
echo.
echo Quick start:
echo   1. Open a new command prompt
echo   2. Run: mindx start
echo   3. Open Dashboard: mindx dashboard
echo   4. Visit: http://localhost:911
echo.
echo To uninstall:
echo   %MINDX_PATH%\uninstall.bat
echo.

pause
