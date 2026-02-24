@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

:: MindX Windows 卸载脚本

echo ========================================
echo   MindX Windows 卸载脚本

echo ========================================
echo.

:: 设置路径
set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

:: 将安装路径设置为当前目录
set "MINDX_PATH=%SCRIPT_DIR%"

:: 获取当前用户名
for /f "delims=" %%i in ('whoami /user /nh') do set "USER_INFO=%%i"
for /f "tokens=2 delims==" %%i in ('echo %USER_INFO%') do set "USER_SID=%%i"
for /f "tokens=* delims= " %%i in ('whoami') do set "FULL_USER=%%i"
for /f "tokens=3 delims=\\" %%i in ('echo %FULL_USER%') do set "CURRENT_USER=%%i"
if "%CURRENT_USER%"=="" set "CURRENT_USER=%USERNAME%"

:: 设置工作目录路径为安装目录/workspaces/用户名
set "MINDX_WORKSPACE=%MINDX_PATH%\workspaces\%CURRENT_USER%"

echo 这将从您的系统中卸载 MindX。
echo.
echo 安装路径: %MINDX_PATH%
echo 工作目录: %MINDX_WORKSPACE%
echo 当前用户: %CURRENT_USER%
echo.
echo 警告: 这将删除所有 MindX 文件和配置。
echo 您的工作区数据将被保留，除非您选择删除它。
echo.

set /p CONFIRM="您确定要卸载吗？(y/n): "
if /i not "%CONFIRM%"=="y" (
    echo 卸载已取消。
    pause
    exit /b 0
)

echo.

:: 询问是否删除工作目录
set /p DELETE_WORKSPACE="是否也删除工作目录？(y/n): "

echo.

:: 删除安装目录
echo 正在删除安装文件...
if exist "%MINDX_PATH%" (
    rmdir /s /q "%MINDX_PATH%" 2>nul
    if exist "%MINDX_PATH%" (
        echo [警告] 无法删除所有文件。有些文件可能正在使用中。
        echo 请关闭所有 MindX 进程并重试。
    ) else (
        echo [OK] 已删除 %MINDX_PATH%
    )
) else (
    echo [信息] 未找到安装目录。
)

:: 从 PATH 环境变量中移除 MindX
echo.
echo 正在从系统 PATH 中移除 MindX...

powershell -NoProfile -ExecutionPolicy Bypass -Command "
$installPath = '%MINDX_PATH%\bin';
$currentPath = [Environment]::GetEnvironmentVariable('PATH', 'Machine');
if ($currentPath.Contains($installPath)) {
    $newPath = $currentPath.Replace($installPath + ';', '').Replace($installPath, '');
    [Environment]::SetEnvironmentVariable('PATH', $newPath, 'Machine');
    Write-Host '[OK] 已从系统 PATH 中移除 MindX';
} else {
    Write-Host '[信息] 在系统 PATH 中未找到 MindX';
}
"

:: 移除 Windows 服务
echo.
echo 正在移除 MindX Windows 服务...

powershell -NoProfile -ExecutionPolicy Bypass -Command "
try {
    $serviceName = 'MindX';
    
    # 检查服务是否存在
    $existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue;
    if ($existingService) {
        # 如果服务正在运行，则停止它
        if ($existingService.Status -eq 'Running') {
            Stop-Service -Name $serviceName;
            Write-Host '[OK] MindX 服务已停止';
        }
        
        # 移除服务
        sc.exe delete $serviceName;
        Write-Host '[OK] MindX 服务已移除';
    } else {
        Write-Host '[信息] 未找到 MindX 服务';
    }
} catch {
    Write-Host '[警告] 无法移除 MindX 服务:' $_.Exception.Message;
}
"

:: 如果请求，移除工作目录
if /i "%DELETE_WORKSPACE%"=="y" (
    echo.
    echo 正在移除工作目录...
    if exist "%MINDX_WORKSPACE%" (
        rmdir /s /q "%MINDX_WORKSPACE%" 2>nul
        if exist "%MINDX_WORKSPACE%" (
            echo [警告] 无法删除所有工作目录文件。
        ) else (
            echo [OK] 已删除 %MINDX_WORKSPACE%
        )
    ) else (
        echo [信息] 未找到工作目录。
    )
) else (
    echo.
    echo [信息] 工作目录已保留在: %MINDX_WORKSPACE%
)

echo.
echo ========================================
echo   卸载完成!
echo ========================================
echo.
echo MindX 已从您的系统中移除。
echo.
echo [OK] MindX 已从系统 PATH 中移除
echo [OK] MindX 服务已移除（如果存在）
echo.
echo 要重新安装 MindX，请再次运行安装脚本。
echo.

pause
