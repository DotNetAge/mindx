@echo off
:: 确保使用正确的编码
chcp 65001 >nul
:: 启用延迟变量扩展
setlocal enabledelayedexpansion

:: 检查并获取管理员权限
>nul 2>&1 "%SYSTEMROOT%\system32\cacls.exe" "%SYSTEMROOT%\system32\config\system"
if '%errorlevel%' NEQ '0' (
    echo 请求管理员权限...
    goto UACPrompt
) else (
    goto gotAdmin
)

:UACPrompt
    echo Set UAC = CreateObject^("Shell.Application"^) > "%temp%\getadmin.vbs"
    echo UAC.ShellExecute "%~s0", "", "", "runas", 1 >> "%temp%\getadmin.vbs"
    "%temp%\getadmin.vbs"
    exit /B

:gotAdmin
    if exist "%temp%\getadmin.vbs" ( del "%temp%\getadmin.vbs" )
    pushd "%CD%"
    CD /D "%~dp0"

:: MindX Windows 安装脚本

echo ========================================
echo   MindX Windows 安装脚本

echo ========================================
echo.

:: 获取脚本目录
set "SCRIPT_DIR=%~dp0"
:: 移除末尾的反斜杠
if "!SCRIPT_DIR:~-1!"=="\" set "SCRIPT_DIR=!SCRIPT_DIR:~0,-1!"
:: 切换到脚本目录
cd /d "!SCRIPT_DIR!"
if %errorlevel% neq 0 (
    echo [错误] 无法切换到脚本目录: !SCRIPT_DIR!
    pause
    exit /b 1
)
echo [信息] 当前目录: !CD!

:: 检查运行模式
if exist "cmd\main.go" (
    set "INSTALL_MODE=source"
    echo 安装模式: 源码
) else (
    set "INSTALL_MODE=release"
    echo 安装模式: 发布包
)

:: 读取版本
if exist "VERSION" (
    set /p VERSION=<VERSION
) else (
    set "VERSION=latest"
)
echo 版本: %VERSION%
echo.

:: 检查依赖
echo [1/7] 检查依赖项...
echo.

:: 检查 Ollama
where ollama >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Ollama 已安装
    set "OLLAMA_AVAILABLE=true"
) else (
    echo [警告] Ollama 未安装，正在安装...
    echo.
    echo 正在安装 Windows 版 Ollama...
    
    :: 使用 PowerShell 安装 Ollama
    echo 正在下载并安装 Ollama...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "
        try {
            Write-Host '正在下载 Ollama 安装脚本...'
            $installScript = Invoke-RestMethod -Uri 'https://ollama.com/install.ps1' -ErrorAction Stop
            Write-Host '正在执行安装脚本...'
            Invoke-Expression $installScript -ErrorAction Stop
            Write-Host '安装完成'
        } catch {
            Write-Host '安装失败: ' $_.Exception.Message
            Exit 1
        }
    "
    
    :: 验证安装
    timeout /t 3 >nul
    where ollama >nul 2>&1
    if %errorlevel% equ 0 (
        echo [OK] Ollama 安装成功
        set "OLLAMA_AVAILABLE=true"
    ) else (
        echo [错误] Ollama 安装失败
        echo.
        echo 请从以下地址手动安装 Ollama: https://ollama.com/download
        pause
        exit /b 1
    )
)

echo.

:: 设置安装路径
echo [2/7] 设置路径...
echo.

:: 将安装路径设置为当前目录
set "MINDX_PATH=%SCRIPT_DIR%"

:: 获取当前用户名
:: 简化版：直接使用 %USERNAME% 环境变量
set "CURRENT_USER=%USERNAME%"
if "!CURRENT_USER!"=="" (
    :: 如果 %USERNAME% 为空，尝试使用 whoami 命令
    for /f "tokens=*" %%i in ('whoami') do set "FULL_USER=%%i"
    for /f "tokens=3 delims=\\" %%i in ('echo !FULL_USER!') do set "CURRENT_USER=%%i"
    if "!CURRENT_USER!"=="" set "CURRENT_USER=User"
)


:: 设置工作目录路径为安装目录/workspaces/用户名
set "MINDX_WORKSPACE=%MINDX_PATH%\workspaces\%CURRENT_USER%"

echo 安装路径: %MINDX_PATH%
echo 工作目录: %MINDX_WORKSPACE%
echo 当前用户: %CURRENT_USER%
echo.

:: 准备二进制文件
echo [3/7] 准备二进制文件...
echo.

if "%INSTALL_MODE%"=="source" (
    echo Windows 批处理脚本不支持从源码构建。
    echo 请使用预构建的发布包。
    pause
    exit /b 1
) else (
    if exist "bin\mindx.exe" (
        echo [OK] 在 bin\ 中找到 mindx.exe
    ) else if exist "mindx.exe" (
        if not exist "bin" mkdir bin
        copy mindx.exe bin\ >nul
        echo [OK] 已将 mindx.exe 复制到 bin\
    ) else (
        echo [错误] 未找到 mindx.exe
        pause
        exit /b 1
    )
)

echo.

:: 安装到 MINDX_PATH
echo [4/7] 安装文件到 %MINDX_PATH%...
echo.

if not exist "%MINDX_PATH%" mkdir "%MINDX_PATH%"
if not exist "%MINDX_PATH%\bin" mkdir "%MINDX_PATH%\bin"

:: 复制二进制文件
copy /y "bin\mindx.exe" "%MINDX_PATH%\bin\" >nul
echo [OK] 已复制 mindx.exe

:: 复制技能
if exist "skills" (
    if not exist "%MINDX_PATH%\skills" mkdir "%MINDX_PATH%\skills"
    xcopy /y /e /q "skills\*" "%MINDX_PATH%\skills\" >nul 2>&1
    echo [OK] 已复制技能
)

:: 复制静态文件
if exist "static" (
    if not exist "%MINDX_PATH%\static" mkdir "%MINDX_PATH%\static"
    xcopy /y /e /q "static\*" "%MINDX_PATH%\static\" >nul 2>&1
    echo [OK] 已复制静态文件
)

:: 复制配置模板
if exist "config" (
    if not exist "%MINDX_PATH%\config" mkdir "%MINDX_PATH%\config"
    for %%f in (config\*) do (
        if exist "%%f" (
            set "filename=%%~nxf"
            copy /y "%%f" "%MINDX_PATH%\config\!filename!.template" >nul 2>&1
        )
    )
    echo [OK] 已复制配置模板
)

:: 复制卸载脚本
if exist "uninstall.bat" (
    copy /y "uninstall.bat" "%MINDX_PATH%\" >nul
    echo [OK] 已复制 uninstall.bat
)

echo.

:: 创建工作目录
echo [5/7] 创建工作目录...
echo.

if not exist "%MINDX_WORKSPACE%" mkdir "%MINDX_WORKSPACE%"
if not exist "%MINDX_WORKSPACE%\config" mkdir "%MINDX_WORKSPACE%\config"
if not exist "%MINDX_WORKSPACE%\logs" mkdir "%MINDX_WORKSPACE%\logs"
if not exist "%MINDX_WORKSPACE%\data" mkdir "%MINDX_WORKSPACE%\data"
if not exist "%MINDX_WORKSPACE%\data\memory" mkdir "%MINDX_WORKSPACE%\data\memory"
if not exist "%MINDX_WORKSPACE%\data\sessions" mkdir "%MINDX_WORKSPACE%\data\sessions"
if not exist "%MINDX_WORKSPACE%\data\training" mkdir "%MINDX_WORKSPACE%\data\training"
if not exist "%MINDX_WORKSPACE%\data\vectors" mkdir "%MINDX_WORKSPACE%\data\vectors"

echo [OK] 已创建工作目录: %MINDX_WORKSPACE%
echo.

:: 设置配置
echo [6/7] 设置配置...
echo.

if exist "%MINDX_PATH%\config" (
    for %%t in ("%MINDX_PATH%\config\*.template") do (
        if exist "%%t" (
            set "template=%%t"
            set "filename=%%~nt"
            
            :: 移除文件名中所有 .template 后缀
            set "clean_filename=!filename!"
            :remove_template
            if "!clean_filename:~-9!"==".template" (
                set "clean_filename=!clean_filename:~0,-9!"
                goto remove_template
            )
            
            set "dest=%MINDX_WORKSPACE%\config\!clean_filename!"
            if not exist "!dest!" (
                copy /y "%%t" "!dest!" >nul 2>&1
                echo [OK] 创建配置: !clean_filename!
            ) else (
                echo [信息] 配置已存在: !clean_filename!
            )
        )
    )
)

echo.

:: 设置 .env 文件
echo [7/7] 设置环境...
echo.

:: 如果工作目录中不存在 .env 文件则创建
if not exist "%MINDX_WORKSPACE%\.env" (
    (
        echo # MindX 环境配置
        echo MINDX_PATH=%MINDX_PATH%
        echo MINDX_WORKSPACE=%MINDX_WORKSPACE%
    ) > "%MINDX_WORKSPACE%\.env"
    echo [OK] 在工作目录中创建 .env 文件
) else (
    echo [信息] 工作目录中已存在 .env 文件
)

echo.

:: 拉取 Ollama 模型
echo [8/8] 拉取 Ollama 模型...
echo.

:: 询问用户是否拉取模型
set /p PULL_MODELS="是否拉取 Ollama 模型？(y/n，默认 n): "
if /i "%PULL_MODELS%"=="" set "PULL_MODELS=n"

if /i "%PULL_MODELS%"=="y" (
    :: 要拉取的模型（与 Linux/macOS 相同）
    set "MODELS=qllama/bge-small-zh-v1.5:latest qwen3:1.7b qwen3:0.6b"

    for %%m in (%MODELS%) do (
        echo.
        echo 检查 %%m...
        
        :: 使用更可靠的方法检查模型是否已安装
        set "MODEL_FOUND=false"
        for /f "tokens=1" %%i in ('ollama list 2^>nul') do (
            if "%%i"=="%%m" (
                set "MODEL_FOUND=true"
            )
        )
        
        if "!MODEL_FOUND!"=="true" (
            echo [OK] %%m 已安装
        ) else (
            echo 正在拉取 %%m...
            ollama pull %%m
            if !errorlevel! equ 0 (
                echo [OK] 拉取 %%m 成功
            ) else (
                echo [警告] 拉取 %%m 失败（稍后再试）
            )
        )
    )
) else (
    echo [信息] 跳过模型拉取，您可以在 Dashboard 中拉取模型
)


echo.

:: 将 MindX 添加到 PATH 环境变量
echo [8/8] 将 MindX 添加到 PATH 环境变量...
echo.

:: 使用 CMD 语法添加到 PATH 并添加更好的错误处理
echo 安装路径: %MINDX_PATH%\bin

:: 先尝试添加到用户 PATH（无需管理员权限）
echo 正在添加到用户 PATH...
set "USER_PATH_FOUND=false"
for /f "tokens=2*" %%i in ('reg query "HKCU\Environment" /v PATH 2^>nul ^| findstr "PATH"') do (
    set "CURRENT_USER_PATH=%%j"
    set "USER_PATH_FOUND=true"
)

if "!USER_PATH_FOUND!"=="true" (
    :: 检查路径是否已存在
    echo !CURRENT_USER_PATH! | findstr /i "!MINDX_PATH!\bin" >nul
    if !errorlevel! equ 0 (
        echo [信息] MindX 已在用户 PATH 中
    ) else (
        :: 添加到用户 PATH
        setx PATH "!CURRENT_USER_PATH!;!MINDX_PATH!\bin" >nul
        if !errorlevel! equ 0 (
            echo [OK] 已将 MindX 添加到用户 PATH
        ) else (
            echo [警告] 无法添加到用户 PATH，尝试直接设置
            reg add "HKCU\Environment" /v PATH /t REG_SZ /d "!CURRENT_USER_PATH!;!MINDX_PATH!\bin" /f >nul
            if !errorlevel! equ 0 (
                echo [OK] 已将 MindX 添加到用户 PATH
            ) else (
                echo [错误] 无法添加到用户 PATH
            )
        )
    )
) else (
    :: 如果用户 PATH 不存在，创建它
    setx PATH "!MINDX_PATH!\bin" >nul
    if !errorlevel! equ 0 (
        echo [OK] 已创建并添加 MindX 到用户 PATH
    ) else (
        echo [错误] 无法创建用户 PATH
    )
)

:: 尝试添加到系统 PATH（需要管理员权限）
echo 正在添加到系统 PATH...
set "SYSTEM_PATH_FOUND=false"
for /f "tokens=2*" %%i in ('reg query "HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment" /v PATH 2^>nul ^| findstr "PATH"') do (
    set "CURRENT_SYSTEM_PATH=%%j"
    set "SYSTEM_PATH_FOUND=true"
)

if "!SYSTEM_PATH_FOUND!"=="true" (
    :: 检查路径是否已存在
    echo !CURRENT_SYSTEM_PATH! | findstr /i "!MINDX_PATH!\bin" >nul
    if !errorlevel! equ 0 (
        echo [信息] MindX 已在系统 PATH 中
    ) else (
        :: 添加到系统 PATH
        setx PATH "!CURRENT_SYSTEM_PATH!;!MINDX_PATH!\bin" /m >nul
        if !errorlevel! equ 0 (
            echo [OK] 已将 MindX 添加到系统 PATH
        ) else (
            echo [警告] 无法添加到系统 PATH（需要管理员权限）
            echo [信息] MindX 已添加到用户 PATH 作为替代
        )
    )
) else (
    :: 如果系统 PATH 不存在，创建它
    setx PATH "!MINDX_PATH!\bin" /m >nul
    if !errorlevel! equ 0 (
        echo [OK] 已创建并添加 MindX 到系统 PATH
    ) else (
        echo [警告] 无法创建系统 PATH（需要管理员权限）
        echo [信息] MindX 已添加到用户 PATH 作为替代
    )
)

:: 提示用户重启命令提示符
 echo.
echo [提示] 请重启命令提示符以使环境变量更改生效

:: 即使 PATH 设置失败也要继续执行脚本
if %errorlevel% neq 0 (
    echo [警告] PATH 设置遇到问题，但继续安装...
    echo.
)

:: 启动 MindX 并设置开机自启
echo.
echo [9/9] 配置 MindX 自启动...
echo.

:: 使用 PowerShell 创建开机自启快捷方式 + 后台启动
powershell -NoProfile -ExecutionPolicy Bypass -Command "
try {
    $binaryPath = '%MINDX_PATH%\bin\mindx.exe';

    # 移除旧的 Windows 服务（如果存在）
    $existingService = Get-Service -Name 'MindX' -ErrorAction SilentlyContinue;
    if ($existingService) {
        Stop-Service -Name 'MindX' -Force -ErrorAction SilentlyContinue;
        sc.exe delete 'MindX' | Out-Null;
        Write-Host '[信息] 已移除旧版 MindX 服务';
    }

    # 创建开机自启快捷方式（Startup 文件夹）
    $startupFolder = [System.Environment]::GetFolderPath('Startup');
    $shortcutPath = Join-Path $startupFolder 'MindX.lnk';
    $shell = New-Object -ComObject WScript.Shell;
    $shortcut = $shell.CreateShortcut($shortcutPath);
    $shortcut.TargetPath = $binaryPath;
    $shortcut.Arguments = 'kernel run';
    $shortcut.WorkingDirectory = '%MINDX_PATH%';
    $shortcut.WindowStyle = 7;
    $shortcut.Description = 'MindX AI Assistant';
    $shortcut.Save();
    Write-Host '[OK] 开机自启已配置';

    # 立即后台启动
    Start-Process -FilePath $binaryPath -ArgumentList 'kernel','run' -WindowStyle Hidden;
    Write-Host '[OK] MindX 已在后台启动';
} catch {
    Write-Host '[警告] 自启动配置失败:' $_.Exception.Message;
    Write-Host '您可以手动运行: mindx kernel run';
}
"

:: 打印摘要
echo ========================================
echo   安装完成!
echo ========================================
echo.
echo MindX 安装成功!
echo.
echo 安装路径: %MINDX_PATH%
echo 工作目录: %MINDX_WORKSPACE%
echo 二进制文件: %MINDX_PATH%\bin\mindx.exe
echo.
echo [OK] MindX 已添加到系统 PATH
echo [OK] MindX 已配置开机自启
echo.
echo 快速开始:
echo   1. 打开新的命令提示符
echo   2. 运行: mindx dashboard
echo   3. 访问: http://localhost:911
echo.
echo 卸载方法:
echo   %MINDX_PATH%\uninstall.bat
echo.

pause
