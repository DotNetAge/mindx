#!/bin/bash

# MindX 开发环境启动脚本
# 用于本地开发,自动启动后端和前端

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo "=== MindX 开发环境启动脚本 ==="
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 开发模式使用的临时工作目录
DEV_WORKSPACE="$PROJECT_ROOT/.dev"
echo -e "${BLUE}开发工作目录: ${DEV_WORKSPACE}${NC}"
echo ""

# 创建并初始化开发工作目录
if [ ! -d "$DEV_WORKSPACE" ]; then
    echo -e "${YELLOW}创建开发工作目录...${NC}"
    mkdir -p "$DEV_WORKSPACE"
    mkdir -p "$DEV_WORKSPACE/config"
    mkdir -p "$DEV_WORKSPACE/logs"
    mkdir -p "$DEV_WORKSPACE/data"
    mkdir -p "$DEV_WORKSPACE/data/memory"
    mkdir -p "$DEV_WORKSPACE/data/sessions"
    mkdir -p "$DEV_WORKSPACE/data/training"
    mkdir -p "$DEV_WORKSPACE/data/vectors"
    
    # 复制配置模板
    if [ -d "config" ]; then
        cp -r config/* "$DEV_WORKSPACE/config/" 2>/dev/null || true
    fi
    
    echo -e "${GREEN}✓ 开发工作目录已初始化${NC}"
    echo ""
fi

# 设置 MINDX_WORKSPACE 环境变量为开发目录
export MINDX_WORKSPACE="$DEV_WORKSPACE"
echo -e "${BLUE}MINDX_WORKSPACE=${MINDX_WORKSPACE}${NC}"
echo ""

# 检查服务器是否运行在911端口
check_server() {
    if lsof -Pi :911 -sTCP:LISTEN -t >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# 检查前端是否运行在5173端口
check_frontend() {
    if lsof -Pi :5173 -sTCP:LISTEN -t >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# 启动后端服务器
start_server() {
    echo -e "${BLUE}[后端]${NC} 正在启动 MindX 服务器..."

    # 强制清理 Badger 锁文件和占用进程（使用开发目录）
    rm -f "$DEV_WORKSPACE/data/vectors/MANIFEST" "$DEV_WORKSPACE/data/vectors/LOCK" 2>/dev/null
    lsof +D "$DEV_WORKSPACE/data/vectors" 2>/dev/null | awk 'NR>1 {print $2}' | xargs -r kill -9 2>/dev/null
    sleep 1

    # 设置开发模式环境变量
    export DEV_MODE=true

    # 使用 go run 启动,支持热重载
    go run ./cmd/main.go kernel run &
    SERVER_PID=$!
    
    echo -e "${BLUE}[后端]${NC} 服务器已启动 (PID: $SERVER_PID)"
    echo -e "${BLUE}[后端]${NC} 等待服务器启动..."

    # 等待端口就绪（最多60秒），而非固定 sleep
    for i in $(seq 1 60); do
        if check_server; then
            break
        fi
        sleep 1
    done
    
    if check_server; then
        echo -e "${GREEN}[后端]${NC} ✓ 服务器运行正常"
        echo -e "  - Dashboard: http://localhost:911"
        echo -e "  - WebSocket: ws://localhost:1314"
        echo -e "  - TUI: mindx tui"
        echo ""
        return 0
    else
        echo -e "${RED}[后端]${NC} ✗ 服务器启动失败"
        echo "请检查错误日志"
        return 1
    fi
}

# 启动前端开发服务器
start_frontend() {
    echo -e "${BLUE}[前端]${NC} 正在启动前端开发服务器..."
    
    cd dashboard
    
    # 检查 node_modules 是否存在
    if [ ! -d "node_modules" ]; then
        echo -e "${YELLOW}[前端]${NC} 首次运行,正在安装依赖..."
        npm install
    fi
    
    # 启动 Vite 开发服务器
    npm run dev &
    FRONTEND_PID=$!
    
    cd ..
    
    echo -e "${BLUE}[前端]${NC} 开发服务器已启动 (PID: $FRONTEND_PID)"
    echo -e "${BLUE}[前端]${NC} 等待前端启动..."
    sleep 5
    
    if check_frontend; then
        echo -e "${GREEN}[前端]${NC} ✓ 前端运行正常"
        echo -e "  - Dev Server: http://localhost:5173"
        echo ""
        return 0
    else
        echo -e "${RED}[前端]${NC} ✗ 前端启动失败"
        echo "请检查错误日志"
        return 1
    fi
}

# 清理函数
cleanup() {
    echo ""
    echo -e "${YELLOW}收到中断信号,正在停止服务...${NC}"
    
    # 停止后端
    if [ ! -z "$SERVER_PID" ]; then
        echo -e "${BLUE}[后端]${NC} 停止服务器 (PID: $SERVER_PID)"
        kill $SERVER_PID 2>/dev/null
    fi
    
    # 停止前端
    if [ ! -z "$FRONTEND_PID" ]; then
        echo -e "${BLUE}[前端]${NC} 停止开发服务器 (PID: $FRONTEND_PID)"
        kill $FRONTEND_PID 2>/dev/null
    fi
    
    # 清理所有相关进程
    pkill -f "mindx kernel" 2>/dev/null
    pkill -f "vite" 2>/dev/null
    
    echo -e "${GREEN}所有服务已停止${NC}"
    exit 0
}

# 设置信号处理
trap cleanup SIGINT SIGTERM

# 主流程
echo -e "${BLUE}检查端口占用...${NC}"
echo ""

SERVER_RUNNING=false
FRONTEND_RUNNING=false

if check_server; then
    echo -e "${YELLOW}[后端]${NC} ✓ 检测到后端服务器已在运行 (端口 911)"
    SERVER_RUNNING=true
else
    echo -e "${BLUE}[后端]${NC} 后端服务器未运行"
fi

if check_frontend; then
    echo -e "${YELLOW}[前端]${NC} ✓ 检测到前端开发服务器已在运行 (端口 5173)"
    FRONTEND_RUNNING=true
else
    echo -e "${BLUE}[前端]${NC} 前端开发服务器未运行"
fi

echo ""

# 询问用户操作
if [ "$SERVER_RUNNING" = true ] || [ "$FRONTEND_RUNNING" = true ]; then
    echo "检测到已有服务运行:"
    [ "$SERVER_RUNNING" = true ] && echo "  ✓ 后端服务器 (端口 911)"
    [ "$FRONTEND_RUNNING" = true ] && echo "  ✓ 前端开发服务器 (端口 5173)"
    echo ""
    
    read -p "是否要重启所有服务? (y/n): " -n 1 -r
    echo
    echo ""
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}正在停止现有服务...${NC}"
        pkill -f "mindx kernel" 2>/dev/null
        pkill -f "vite" 2>/dev/null
        sleep 3
        
        # 清理 Badger 锁文件（使用开发目录）
        rm -f "$DEV_WORKSPACE/data/vectors/MANIFEST" "$DEV_WORKSPACE/data/vectors/LOCK" 2>/dev/null
    else
        echo -e "${GREEN}保持现有服务运行${NC}"
        
        # 只启动未运行的服务
        if [ "$SERVER_RUNNING" = false ]; then
            echo ""
            start_server
        fi
        if [ "$FRONTEND_RUNNING" = false ]; then
            start_frontend
        fi
        
        echo ""
        echo -e "${GREEN}=== 开发环境已就绪 ===${NC}"
        echo -e "后端: ${check_server && echo '✓ 运行中' || echo '✗ 未运行'}"
        echo -e "前端: ${check_frontend && echo '✓ 运行中' || echo '✗ 未运行'}"
        echo ""
        echo -e "提示: 按 ${YELLOW}Ctrl+C${NC} 停止服务"
        echo ""
        
        # 如果服务都没启动,则退出
        if [ "$SERVER_RUNNING" = false ] && [ "$FRONTEND_RUNNING" = false ]; then
            exit 1
        fi
        
        wait
        exit 0
    fi
fi

# 启动所有服务
echo -e "${BLUE}启动所有服务...${NC}"
echo ""

start_success=true

if ! start_server; then
    start_success=false
fi

if ! start_frontend; then
    start_success=false
fi

if [ "$start_success" = true ]; then
    echo ""
    echo -e "${GREEN}=== 开发环境已就绪 ===${NC}"
    echo ""
    echo "服务访问地址:"
    echo "  - 后端 API: http://localhost:911"
    echo "  - 前端界面: http://localhost:5173"
    echo "  - WebSocket: ws://localhost:1314"
    echo "  - TUI: mindx tui"
    echo ""
    echo "开发提示:"
    echo "  - 后端代码修改后需要手动重启"
    echo "  - 前端代码修改会自动热重载"
    echo "  - 查看 http://localhost:5173 进行开发"
    echo ""
    echo -e "提示: 按 ${YELLOW}Ctrl+C${NC} 停止所有服务"
    echo ""
else
    echo ""
    echo -e "${RED}=== 启动失败 ===${NC}"
    echo "请检查错误日志并重试"
    cleanup
fi

# 等待后台进程
wait
