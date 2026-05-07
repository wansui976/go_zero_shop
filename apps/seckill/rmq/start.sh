#!/bin/sh
set -e

echo "Starting Seckill RMQ Consumers..."

./order-consumer -f etc/seckill-consumer.yaml &
ORDER_PID=$!

./stock-consumer -f etc/stock-consumer.yaml &
STOCK_PID=$!

./delay-consumer -f etc/delay-consumer.yaml &
DELAY_PID=$!

echo "All seckill consumers started."
echo "  order-consumer PID: $ORDER_PID"
echo "  stock-consumer PID: $STOCK_PID"
echo "  delay-consumer PID: $DELAY_PID"

wait -n
exit 1
