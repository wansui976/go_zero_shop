#!/bin/bash

# 简化版微服务启动脚本
echo "开始启动微服务系统..."

# 创建日志目录
mkdir -p logs


echo "按依赖顺序启动所有服务..."

# 1. User RPC (8080)
echo "启动 User RPC (8080)..."
cd apps/user/rpc
nohup go run user.go -f etc/user.yaml > ../../../logs/user-rpc.log 2>&1 &
USER_PID=$!
cd - > /dev/null
sleep 3

# 2. Product RPC (8081)  
echo "启动 Product RPC (8081)..."
cd apps/product/rpc
nohup go run product.go -f etc/product.yaml > ../../../logs/product-rpc.log 2>&1 &
PRODUCT_PID=$!
cd - > /dev/null
sleep 3

# 3. Order RPC (8082)
echo "启动 Order RPC (8082)..."
cd apps/order/rpc
nohup go run order.go -f etc/order.yaml > ../../../logs/order-rpc.log 2>&1 &
ORDER_PID=$!
cd - > /dev/null
sleep 3

#4. Reply RPC (8083)
echo "启动 Search RPC (8083)..."
cd apps/search/rpc
nohup go run search.go -f etc/search.yaml > ../../../logs/search-rpc.log 2>&1 &
REPLY_PID=$!
cd - > /dev/null
sleep 3

# 4. Cart RPC (8084)
echo "启动 Cart RPC (8084)..."
cd apps/cart/rpc
nohup go run cart.go -f etc/cart.yaml > ../../../logs/cart-rpc.log 2>&1 &
CART_PID=$!
cd - > /dev/null
sleep 3


# 5. Seckill RPC (9889)
echo "启动 Seckill RPC (9889)..."
cd apps/seckill/rpc
nohup go run seckill.go -f etc/seckill.yaml > ../../../logs/seckill-rpc.log 2>&1 &
SECKILL_PID=$!
cd - > /dev/null
sleep 3

# 6. API Gateway (8888)
echo "启动 API Gateway (8888)..."
cd apps/app/api
nohup go run api.go -f etc/api-api.yaml > ../../../logs/api-gateway.log 2>&1 &
API_PID=$!
cd - > /dev/null
sleep 3

echo ""
echo "验证服务状态..."

# 检查所有服务是否正常运行
check_service() {
    local service_name=$1
    local pid=$2
    local port=$3
    
    if kill -0 $pid 2>/dev/null && lsof -i :$port > /dev/null 2>&1; then
        echo "✅ $service_name (PID: $pid, Port: $port) - 运行正常"
        return 0
    else
        echo "❌ $service_name (PID: $pid, Port: $port) - 启动失败"
        return 1
    fi
}

SUCCESS_COUNT=0

check_service "User RPC" $USER_PID 8080 && ((SUCCESS_COUNT++))
check_service "Product RPC" $PRODUCT_PID 8081 && ((SUCCESS_COUNT++))
check_service "Order RPC" $ORDER_PID 8082 && ((SUCCESS_COUNT++))
check_service "Search RPC" $REPLY_PID 8083 && ((SUCCESS_COUNT++))
check_service "Seckill RPC" $SECKILL_PID 9889 && ((SUCCESS_COUNT++))
check_service "Cart RPC" $CART_PID 8084 && ((SUCCESS_COUNT++))
check_service "API Gateway" $API_PID 8888 && ((SUCCESS_COUNT++))

echo ""
if [ $SUCCESS_COUNT -eq 7 ]; then
    echo "🎉 所有 7 个微服务启动成功！"
    echo "日志目录: logs/"
    echo "使用 ./stop_all_services.sh 停止所有服务"
else
    echo "❌ 部分服务启动失败 ($SUCCESS_COUNT/7)"
    echo "请检查 logs/ 目录下的日志文件"
    
    echo ""
    echo "最近的错误日志:"
    for log_file in logs/*.log; do
        if [ -f "$log_file" ]; then
            echo "=== $log_file ==="
            tail -10 "$log_file"
            echo ""
        fi
    done
fi