#!/bin/bash
set -e

NAMESPACE="go-zero-shop"
SERVICES=("api-gateway" "user-rpc" "product-rpc" "reply-rpc" "cart-rpc" "pay-rpc" "search-rpc" "order-rpc" "seckill-rpc" "dtm" "mysql" "redis" "etcd" "rabbitmq" "kafka" "elasticsearch" "jaeger" "prometheus" "grafana" "kibana")

show_help() {
  echo "Go-Zero Shop Kubernetes 管理脚本"
  echo ""
  echo "使用方法: ./k8s-manage.sh [命令]"
  echo ""
  echo "可用命令:"
  echo "  status      - 查看所有服务状态"
  echo "  logs        - 查看核心服务日志"
  echo "  restart     - 重启核心业务服务"
  echo "  scale       - 扩缩容核心业务服务"
  echo "  test        - 测试 API Gateway 连通性"
  echo "  clean       - 清理所有资源"
  echo "  help        - 显示帮助信息"
}

show_status() {
  echo "📊 查看服务状态..."
  kubectl get pods -n ${NAMESPACE} -o wide
  echo
  kubectl get services -n ${NAMESPACE}
  echo
  kubectl get deployments -n ${NAMESPACE}
}

show_logs() {
  echo "📝 查看核心服务日志..."
  for service in api-gateway user-rpc product-rpc reply-rpc cart-rpc pay-rpc search-rpc order-rpc dtm prometheus grafana kibana; do
    echo "
=== ${service} 日志 ==="
    kubectl logs --tail=40 deployment/${service} -n ${NAMESPACE} || echo "❌ 无法获取 ${service} 日志"
  done
}

restart_services() {
  echo "🔄 重启核心业务服务..."
  for service in api-gateway user-rpc product-rpc reply-rpc cart-rpc pay-rpc search-rpc order-rpc seckill-rpc; do
    kubectl rollout restart deployment/${service} -n ${NAMESPACE}
    kubectl rollout status deployment/${service} -n ${NAMESPACE}
  done
}

scale_services() {
  echo "请输入副本数量 (默认2): "
  read -r replicas
  replicas=${replicas:-2}
  for service in user-rpc product-rpc reply-rpc cart-rpc pay-rpc search-rpc order-rpc seckill-rpc api-gateway; do
    kubectl scale deployment ${service} --replicas=${replicas} -n ${NAMESPACE}
  done
  echo "✅ 扩缩容完成"
}

test_connectivity() {
  echo "🔧 测试 API Gateway 连通性..."
  api_service=$(kubectl get service api-gateway -n ${NAMESPACE} -o jsonpath='{.spec.clusterIP}')
  if [[ -n "${api_service}" ]]; then
    kubectl run test-pod --image=curlimages/curl --rm -it --restart=Never -n ${NAMESPACE} -- curl -sS http://${api_service}:8888/ping || echo "❌ API Gateway 连通性测试失败"
  else
    echo "❌ 无法获取 API Gateway 服务 IP"
  fi
  echo "kubectl port-forward service/api-gateway 8888:8888 -n ${NAMESPACE}"
}

clean_resources() {
  echo "⚠️  这将删除命名空间 ${NAMESPACE} 中的所有资源"
  echo "确认删除？(y/N): "
  read -r confirm
  if [[ "${confirm}" == "y" || "${confirm}" == "Y" ]]; then
    kubectl delete namespace ${NAMESPACE}
  else
    echo "❌ 取消清理"
  fi
}

case "${1:-help}" in
  status) show_status ;;
  logs) show_logs ;;
  restart) restart_services ;;
  scale) scale_services ;;
  test) test_connectivity ;;
  clean) clean_resources ;;
  help|"") show_help ;;
  *) echo "❌ 未知命令: ${1}"; show_help; exit 1 ;;
esac
