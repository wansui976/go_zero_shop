#!/bin/bash

# K8s 部署脚本
set -e

echo "🚀 开始部署 Go-Zero Shop 微服务到 Kubernetes..."

# 检查 kubectl 是否可用
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl 未安装，请先安装 kubectl"
    exit 1
fi

# 检查集群连接
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ 无法连接到 Kubernetes 集群"
    exit 1
fi

echo "✅ Kubernetes 集群连接正常"

# 1. 创建命名空间
echo "📦 创建命名空间..."
kubectl apply -f k8s/namespace.yaml

# 2. 创建配置文件
echo "📝 创建配置文件..."
kubectl apply -f k8s/configs/

# 3. 部署基础设施（按顺序）
echo "🏗️  部署基础设施..."
echo "部署 MySQL..."
kubectl apply -f k8s/infrastructure/mysql.yaml

echo "部署 Redis..."
kubectl apply -f k8s/infrastructure/redis.yaml

echo "部署 Etcd..."
kubectl apply -f k8s/infrastructure/etcd.yaml

echo "部署 Kafka..."
kubectl apply -f k8s/infrastructure/kafka.yaml

echo "部署 DTM..."
kubectl apply -f k8s/infrastructure/dtm.yaml

# 等待基础设施启动
echo "⏳ 等待基础设施启动..."
kubectl wait --for=condition=Ready pod -l app=mysql -n go-zero-shop --timeout=300s
kubectl wait --for=condition=Ready pod -l app=redis -n go-zero-shop --timeout=300s
kubectl wait --for=condition=Ready pod -l app=etcd -n go-zero-shop --timeout=300s
kubectl wait --for=condition=Ready pod -l app=kafka -n go-zero-shop --timeout=300s
kubectl wait --for=condition=Ready pod -l app=dtm -n go-zero-shop --timeout=300s

echo "✅ 基础设施部署完成"

# 4. 部署RPC服务（按依赖顺序）
echo "🚀 部署RPC服务..."

echo "部署 User RPC..."
kubectl apply -f k8s/deployments/user-rpc.yaml
kubectl wait --for=condition=Ready pod -l app=user-rpc -n go-zero-shop --timeout=300s

echo "部署 Product RPC..."
kubectl apply -f k8s/deployments/product-rpc.yaml
kubectl wait --for=condition=Ready pod -l app=product-rpc -n go-zero-shop --timeout=300s

echo "部署 Reply RPC..."
kubectl apply -f k8s/deployments/reply-rpc.yaml
kubectl wait --for=condition=Ready pod -l app=reply-rpc -n go-zero-shop --timeout=300s

echo "部署 Order RPC..."
kubectl apply -f k8s/deployments/order-rpc.yaml
kubectl wait --for=condition=Ready pod -l app=order-rpc -n go-zero-shop --timeout=300s

echo "部署 Seckill RPC..."
kubectl apply -f k8s/deployments/seckill-rpc.yaml
kubectl wait --for=condition=Ready pod -l app=seckill-rpc -n go-zero-shop --timeout=300s

echo "✅ RPC服务部署完成"

# 5. 部署API网关
echo "🌐 部署 API 网关..."
kubectl apply -f k8s/deployments/api-gateway.yaml
kubectl wait --for=condition=Ready pod -l app=api-gateway -n go-zero-shop --timeout=300s

echo "✅ API 网关部署完成"

# 6. 显示部署状态
echo "📊 部署状态："
kubectl get pods -n go-zero-shop

echo ""
echo "🌐 服务访问信息："
kubectl get services -n go-zero-shop

echo ""
echo "🎉 部署完成！"
echo ""
echo "📝 接下来的步骤："
echo "1. 等待所有 Pod 运行正常"
echo "2. 通过 LoadBalancer 或 NodePort 访问 API 网关"
echo "3. 使用以下命令查看日志："
echo "   kubectl logs -f deployment/api-gateway -n go-zero-shop"
echo ""
echo "🔧 常用管理命令："
echo "   查看所有资源: kubectl get all -n go-zero-shop"
echo "   查看 Pod 日志: kubectl logs -f <pod-name> -n go-zero-shop"
echo "   进入 Pod: kubectl exec -it <pod-name> -n go-zero-shop -- sh"
echo "   删除所有资源: kubectl delete namespace go-zero-shop"