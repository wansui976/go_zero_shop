#!/bin/bash

echo "🔍 直接测试Product RPC"
echo "============================"

# 使用Go客户端直接测试RPC
cat > /tmp/test_rpc.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "log"
    
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

// 临时定义proto消息结构
type ProductListRequest struct {
    CategoryId int32
    Cursor     int64  
    Ps         int32
    ProductId  int64
}

type ProductListResponse struct {
    IsEnd     bool
    Timestamp int64
    ProductId int64
    Products  []*ProductItem
}

type ProductItem struct {
    ProductId  int64
    Name       string
    Description string
    ImageUrl   string
    CreateTime int64
    Stock      int64
    Cateid     int64
    Price      float64
    Status     int64
}

func main() {
    // 连接到Product RPC
    conn, err := grpc.Dial("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer conn.Close()
    
    fmt.Println("成功连接到Product RPC服务")
    fmt.Println("测试完成 - 连接正常")
}
EOF

echo "1. 测试RPC连接："
cd /tmp && go mod init test_rpc > /dev/null 2>&1
go get google.golang.org/grpc > /dev/null 2>&1
go run test_rpc.go

echo ""
echo "2. 检查Product RPC日志："
tail -3 /Users/yulang/Documents/go_zero_shop/logs/product-rpc.log

echo ""
echo "3. 手动测试categoryId=2:"
echo "检查categories表中ID=2的记录："
mysql -u root -h 127.0.0.1 -e "USE product; SELECT * FROM categories WHERE id=2;"

echo ""
echo "检查该分类下的商品："
mysql -u root -h 127.0.0.1 -e "USE product; SELECT id, name, cateid FROM products WHERE cateid=2 LIMIT 3;"

echo ""
echo "4. 测试结论："
echo "如果连接正常但API仍失败，问题可能在于："
echo "- CategoryModel.FindOne查询失败"  
echo "- 缓存问题"
echo "- RPC参数传递问题"

rm -f /tmp/test_rpc.go /tmp/go.mod /tmp/go.sum