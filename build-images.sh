#!/bin/bash

# 构建Docker镜像脚本
set -e

echo "🐳 开始构建 Docker 镜像..."

# 设置镜像仓库前缀（可根据需要修改）
REGISTRY="go-zero-shop"
TAG="latest"

# 构建所有服务镜像
echo "🔨 构建 User RPC 镜像..."
docker build -f Dockerfile.user -t ${REGISTRY}/user-rpc:${TAG} .

echo "🔨 构建 Product RPC 镜像..."
docker build -f Dockerfile.product -t ${REGISTRY}/product-rpc:${TAG} .

echo "🔨 构建 Order RPC 镜像..."
docker build -f Dockerfile.order -t ${REGISTRY}/order-rpc:${TAG} .

echo "🔨 构建 Reply RPC 镜像..."
docker build -f Dockerfile.reply -t ${REGISTRY}/reply-rpc:${TAG} .

echo "🔨 构建 Seckill RPC 镜像..."
docker build -f Dockerfile.seckill -t ${REGISTRY}/seckill-rpc:${TAG} .

echo "🔨 构建 API Gateway 镜像..."
docker build -f Dockerfile.api -t ${REGISTRY}/api-gateway:${TAG} .

echo "📋 构建完成的镜像列表："
docker images | grep ${REGISTRY}

echo ""
echo "✅ 所有镜像构建完成！"
echo ""
echo "📝 如果需要推送到镜像仓库，请执行："
echo "   docker push ${REGISTRY}/user-rpc:${TAG}"
echo "   docker push ${REGISTRY}/product-rpc:${TAG}"
echo "   docker push ${REGISTRY}/order-rpc:${TAG}"
echo "   docker push ${REGISTRY}/reply-rpc:${TAG}"
echo "   docker push ${REGISTRY}/seckill-rpc:${TAG}"
echo "   docker push ${REGISTRY}/api-gateway:${TAG}"