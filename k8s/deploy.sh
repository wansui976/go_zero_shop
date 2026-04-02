#!/bin/bash
set -euo pipefail

NAMESPACE="go-zero-shop"

wait_deploy() {
  kubectl rollout status deployment/$1 -n ${NAMESPACE} --timeout=300s
}

echo "🚀 开始部署 Go-Zero Shop 到 Kubernetes..."
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl 未安装"; exit 1; }
kubectl cluster-info >/dev/null 2>&1 || { echo "❌ 无法连接到 Kubernetes 集群"; exit 1; }

echo "📦 应用基础资源..."
kubectl apply -f k8s/base/namespace.yaml
kubectl apply -f k8s/base/configmap.yaml
kubectl apply -f k8s/base/secrets.yaml
kubectl apply -f k8s/configs/

echo "🏗️ 部署基础设施..."
for f in mysql redis etcd rabbitmq kafka elasticsearch jaeger dtm prometheus grafana kibana mysqld-exporter redis-exporter rabbitmq-exporter elasticsearch-exporter node-exporter; do
  kubectl apply -f k8s/infrastructure/${f}.yaml
 done

for deploy in mysql redis etcd rabbitmq kafka elasticsearch jaeger dtm prometheus grafana kibana mysqld-exporter redis-exporter rabbitmq-exporter elasticsearch-exporter; do
  wait_deploy ${deploy}
done
kubectl rollout status daemonset/node-exporter -n ${NAMESPACE} --timeout=300s

echo "🧩 部署核心 RPC 服务..."
for f in user-rpc product-rpc reply-rpc cart-rpc pay-rpc search-rpc order-rpc seckill-rpc; do
  kubectl apply -f k8s/deployments/${f}.yaml
 done

for deploy in user-rpc product-rpc reply-rpc cart-rpc pay-rpc search-rpc order-rpc seckill-rpc; do
  wait_deploy ${deploy}
done

echo "🌐 部署 API 网关..."
kubectl apply -f k8s/deployments/api-gateway.yaml
wait_deploy api-gateway

echo "📌 可选：如果已安装 ingress-nginx，可执行："
echo "   kubectl apply -f k8s/base/ingress.yaml"

echo "
✅ 部署完成，当前资源："
kubectl get pods,svc -n ${NAMESPACE}

echo "
🔎 本地联调："
echo "   kubectl port-forward svc/api-gateway 8888:8888 -n ${NAMESPACE}"
