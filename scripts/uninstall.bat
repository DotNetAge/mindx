@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

:: MindX Windows Uninstallation Script

echo ========================================
echo   MindX Windows Uninstallation Script
echo ========================================
echo.

:: Set paths
set "MINDX_PATH=%LOCALAPPDATA%\MindX"
set "MINDX_WORKSPACE=%USERPROFILE%\.mindx"

echo This will uninstall MindX from your system.
echo.
echo Install path: %MINDX_PATH%
echo Workspace:    %MINDX_WORKSPACE%
echo.
echo WARNING: This will remove all MindX files and configurations.
echo Your workspace data will be preserved unless you choose to delete it.
echo.

set /p CONFIRM="Are you sure you want to uninstall? (y/n): "
if /i not "%CONFIRM%"=="y" (
    echo Uninstall cancelled.
    pause
    exit /b 0
)

echo.

:: Ask about workspace
set /p DELETE_WORKSPACE="Delete workspace directory too? (y/n): "

echo.

:: Remove installation directory
echo Removing installation files...
if exist "%MINDX_PATH%" (
    rmdir /s /q "%MINDX_PATH%" 2>nul
    if exist "%MINDX_PATH%" (
        echo [WARN] Could not remove all files. Some may be in use.
        echo Please close any MindX processes and try again.
    ) else (
        echo [OK] Removed %MINDX_PATH%
    )
) else (
    echo [INFO] Installation directory not found.
)

:: Remove workspace if requested
if /i "%DELETE_WORKSPACE%"=="y" (
    echo.
    echo Removing workspace...
    if exist "%MINDX_WORKSPACE%" (
        rmdir /s /q "%MINDX_WORKSPACE%" 2>nul
        if exist "%MINDX_WORKSPACE%" (
            echo [WARN] Could not remove all workspace files.
        ) else (
            echo [OK] Removed %MINDX_WORKSPACE%
        )
    ) else (
        echo [INFO] Workspace directory not found.
    )
) else (
    echo.
    echo [INFO] Workspace preserved at: %MINDX_WORKSPACE%
)

echo.
echo ========================================
echo   Uninstallation Complete!
echo ========================================
echo.
echo MindX has been removed from your system.
echo.
echo NOTE: You may need to manually remove MindX from your PATH:
echo   1. Open System Properties
echo   2. Go to Advanced -^> Environment Variables
echo   3. Edit the "Path" variable and remove: %MINDX_PATH%\bin
echo.

pause
