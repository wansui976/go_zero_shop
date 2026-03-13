#!/bin/bash

# 本地构建 Linux/arm64 二进制，并使用 FROM scratch 构建镜像，直接构建到 minikube docker 守护进程
set -euo pipefail

REGISTRY="go-zero-shop"
TAG="latest"

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)
BIN_DIR="${ROOT_DIR}/bin"

mkdir -p "${BIN_DIR}"

echo "🔧 切换 docker 到 minikube 守护进程"
eval "$(minikube docker-env)"

build_binary() {
  local name="$1"
  local pkg_path="$2"
  local out_path="${BIN_DIR}/${name}"
  echo "🏗️  编译 ${name} -> ${out_path}"
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags='-s -w' -o "${out_path}" "${pkg_path}"
}

build_image() {
  local name="$1"
  local dockerfile="$2"
  echo "🐳 构建镜像 ${REGISTRY}/${name}:${TAG} (Dockerfile: ${dockerfile})"
  docker build -f "${dockerfile}" -t "${REGISTRY}/${name}:${TAG}" .
}

echo "🏗️  开始编译二进制 (linux/arm64)"
build_binary "user-rpc" "${ROOT_DIR}/apps/user/rpc/user.go"
build_binary "product-rpc" "${ROOT_DIR}/apps/product/rpc/product.go"
build_binary "order-rpc" "${ROOT_DIR}/apps/order/rpc/order.go"
build_binary "reply-rpc" "${ROOT_DIR}/apps/reply/rpc/reply.go"
build_binary "seckill-rpc" "${ROOT_DIR}/apps/seckill/rpc/seckill.go"
build_binary "api-gateway" "${ROOT_DIR}/apps/app/api/api.go"

echo "🐳 使用 scratch 打包镜像"
build_image "user-rpc" "${ROOT_DIR}/Dockerfile.user.scratch"
build_image "product-rpc" "${ROOT_DIR}/Dockerfile.product.scratch"
build_image "order-rpc" "${ROOT_DIR}/Dockerfile.order.scratch"
build_image "reply-rpc" "${ROOT_DIR}/Dockerfile.reply.scratch"
build_image "seckill-rpc" "${ROOT_DIR}/Dockerfile.seckill.scratch"
build_image "api-gateway" "${ROOT_DIR}/Dockerfile.api.scratch"

echo "📋 可用镜像："
docker images | grep "${REGISTRY}"

echo "✅ 本地镜像构建完成（已在 minikube docker 内）"




