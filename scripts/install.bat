@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

echo ========================================
echo   MindX 安装脚本
echo ========================================
echo.

set "INSTALL_DIR=%ProgramFiles%\MindX"
set "WORKSPACE=%APPDATA%\MindX"

echo 安装目录: %INSTALL_DIR%
echo 工作目录: %WORKSPACE%
echo.

echo [1/6] 创建安装目录...
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
echo  [OK]

echo [2/6] 复制文件...
xcopy /y /e /q bin\* "%INSTALL_DIR%\bin\" >nul 2>&1
if exist "skills" xcopy /y /e /q skills\* "%INSTALL_DIR%\skills\" >nul 2>&1
if exist "static" xcopy /y /e /q static\* "%INSTALL_DIR%\static\" >nul 2>&1
if exist "config" xcopy /y /e /q config\* "%INSTALL_DIR%\config\" >nul 2>&1
copy /y install.bat "%INSTALL_DIR%\" >nul
copy /y uninstall.bat "%INSTALL_DIR%\" >nul 2>&1
copy /y README* "%INSTALL_DIR%\" >nul 2>&1
copy /y VERSION "%INSTALL_DIR%\" >nul 2>&1
echo  [OK]

echo [3/6] 创建工作目录...
if not exist "%WORKSPACE%" mkdir "%WORKSPACE%"
if not exist "%WORKSPACE%\config" mkdir "%WORKSPACE%\config"
if not exist "%WORKSPACE%\logs" mkdir "%WORKSPACE%\logs"
if not exist "%WORKSPACE%\data" mkdir "%WORKSPACE%\data"
echo  [OK]

echo [4/6] 创建配置文件...
if not exist "%WORKSPACE%\.env" (
    (
        echo MINDX_HOME=%INSTALL_DIR%
        echo MINDX_WORKSPACE=%WORKSPACE%
    ) > "%WORKSPACE%\.env"
)
echo  [OK]

echo [5/6] 设置环境变量...
setx MINDX_HOME "%INSTALL_DIR%" >nul
setx PATH "%INSTALL_DIR%\bin;%PATH%" >nul
echo  [OK] MINDX_HOME
echo  [OK] PATH

echo [6/6] 检查 Ollama...
where ollama >nul 2>&1
if %errorlevel% equ 0 (
    echo  [OK] Ollama 已安装
) else (
    echo  [信息] Ollama 未安装
    echo  请从 https://ollama.com 下载安装
)

echo.
echo ========================================
echo   安装完成!
echo ========================================
echo.
echo 安装目录: %INSTALL_DIR%
echo 工作目录: %WORKSPACE%
echo.
echo 启动 MindX:
echo   %INSTALL_DIR%\bin\mindx.exe kernel run
echo.
echo 提示: 请重新打开命令提示符使环境变量生效
echo.

pause
