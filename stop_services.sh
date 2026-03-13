#!/bin/bash

# ========= 颜色 =========
RED="\033[31m"
GREEN="\033[32m"
BLUE="\033[34m"
RESET="\033[0m"

# ========= 需要停止的服务端口 =========
PORTS=(8888 9889 8084 8083 8082 8081 8080)

echo -e "${BLUE}停止所有微服务...${RESET}"

stop_port() {
    PORT=$1
    PIDS=$(lsof -ti :${PORT})

    if [[ -z "$PIDS" ]]; then
        echo -e "${GREEN}端口 ${PORT}：无运行进程${RESET}"
        return
    fi

    echo -e "${BLUE}停止占用端口 ${PORT} 的进程: ${PIDS}${RESET}"

    
    kill ${PIDS} 2>/dev/null || true

    # 等待退出
    sleep 0.5

    # 如果仍然存在，则强制 kill -9
    PIDS_AFTER=$(lsof -ti :${PORT})
    if [[ -n "$PIDS_AFTER" ]]; then
        echo -e "${RED}端口 ${PORT} 的进程仍未退出，执行 kill -9...${RESET}"
        kill -9 ${PIDS_AFTER} 2>/dev/null || true
    fi

    echo -e "${GREEN}端口 ${PORT} 清理完成${RESET}"
}

# 按顺序关闭所有端口
for PORT in "${PORTS[@]}"; do
    stop_port $PORT
done

echo ""
echo -e "${BLUE}检查端口状态...${RESET}"

for PORT in "${PORTS[@]}"; do
    if lsof -i :${PORT} > /dev/null 2>&1; then
        echo -e "${RED}端口 ${PORT}: 仍被占用${RESET}"
    else
        echo -e "${GREEN}端口 ${PORT}: 已释放${RESET}"
    fi
done

echo ""
echo -e "${GREEN}所有微服务已停止！${RESET}"