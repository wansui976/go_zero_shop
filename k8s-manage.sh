#!/bin/bash

# K8s 管理脚本
set -e

NAMESPACE="go-zero-shop"

function show_help() {
    echo "Go-Zero Shop Kubernetes 管理脚本"
    echo ""
    echo "使用方法: ./k8s-manage.sh [命令]"
    echo ""
    echo "可用命令:"
    echo "  status      - 查看所有服务状态"
    echo "  logs        - 查看所有服务日志"
    echo "  restart     - 重启所有服务"
    echo "  scale       - 扩缩容服务"
    echo "  test        - 测试服务连通性"
    echo "  clean       - 清理所有资源"
    echo "  help        - 显示帮助信息"
    echo ""
}

function show_status() {
    echo "📊 查看服务状态..."
    echo ""
    echo "=== Pods 状态 ==="
    kubectl get pods -n ${NAMESPACE} -o wide
    
    echo ""
    echo "=== Services 状态 ==="
    kubectl get services -n ${NAMESPACE}
    
    echo ""
    echo "=== Deployments 状态 ==="
    kubectl get deployments -n ${NAMESPACE}
}

function show_logs() {
    echo "📝 查看服务日志..."
    
    services=("user-rpc" "product-rpc" "order-rpc" "reply-rpc" "seckill-rpc" "api-gateway")
    
    for service in "${services[@]}"; do
        echo ""
        echo "=== ${service} 日志 ==="
        kubectl logs --tail=20 deployment/${service} -n ${NAMESPACE} || echo "❌ 无法获取 ${service} 日志"
    done
}

function restart_services() {
    echo "🔄 重启所有服务..."
    
    services=("user-rpc" "product-rpc" "order-rpc" "reply-rpc" "seckill-rpc" "api-gateway")
    
    for service in "${services[@]}"; do
        echo "重启 ${service}..."
        kubectl rollout restart deployment/${service} -n ${NAMESPACE}
        kubectl rollout status deployment/${service} -n ${NAMESPACE}
    done
    
    echo "✅ 所有服务重启完成"
}

function scale_services() {
    echo "📈 扩缩容服务..."
    echo "请输入副本数量 (默认2): "
    read -r replicas
    replicas=${replicas:-2}
    
    services=("user-rpc" "product-rpc" "order-rpc" "reply-rpc" "seckill-rpc")
    
    for service in "${services[@]}"; do
        echo "扩缩容 ${service} 到 ${replicas} 个副本..."
        kubectl scale deployment ${service} --replicas=${replicas} -n ${NAMESPACE}
    done
    
    echo "扩缩容 api-gateway 到 3 个副本..."
    kubectl scale deployment api-gateway --replicas=3 -n ${NAMESPACE}
    
    echo "✅ 扩缩容完成"
}

function test_connectivity() {
    echo "🔧 测试服务连通性..."
    
    # 获取 API Gateway 的服务 IP
    api_service=$(kubectl get service api-gateway -n ${NAMESPACE} -o jsonpath='{.spec.clusterIP}')
    
    if [[ -n "${api_service}" ]]; then
        echo "API Gateway 服务 IP: ${api_service}:8888"
        
        # 创建测试 Pod
        kubectl run test-pod --image=curlimages/curl --rm -it --restart=Never -n ${NAMESPACE} -- curl -v http://${api_service}:8888/ping || echo "❌ API Gateway 连通性测试失败"
    else
        echo "❌ 无法获取 API Gateway 服务 IP"
    fi
    
    echo ""
    echo "📋 端口转发命令（本地测试）："
    echo "kubectl port-forward service/api-gateway 8888:8888 -n ${NAMESPACE}"
}

function clean_resources() {
    echo "🗑️  清理所有资源..."
    echo "⚠️  这将删除命名空间 ${NAMESPACE} 中的所有资源"
    echo "确认删除？(y/N): "
    read -r confirm
    
    if [[ "${confirm}" == "y" || "${confirm}" == "Y" ]]; then
        kubectl delete namespace ${NAMESPACE}
        echo "✅ 清理完成"
    else
        echo "❌ 取消清理"
    fi
}

# 主逻辑
case "${1}" in
    "status")
        show_status
        ;;
    "logs")
        show_logs
        ;;
    "restart")
        restart_services
        ;;
    "scale")
        scale_services
        ;;
    "test")
        test_connectivity
        ;;
    "clean")
        clean_resources
        ;;
    "help"|"")
        show_help
        ;;
    *)
        echo "❌ 未知命令: ${1}"
        show_help
        exit 1
        ;;
esac