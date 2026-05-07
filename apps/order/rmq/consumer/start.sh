#!/bin/sh
set -e

echo "Starting Order RMQ Consumers..."

# Start all consumers in background
./order-consumer -f etc/order-consumer.yaml &
ORDER_PID=$!

./stock-consumer -f etc/stock-consumer.yaml &
STOCK_PID=$!

./delay-consumer -f etc/delay-consumer.yaml &
DELAY_PID=$!

echo "All consumers started. Waiting for processes..."
echo "  order-consumer PID: $ORDER_PID"
echo "  stock-consumer PID: $STOCK_PID"
echo "  delay-consumer PID: $DELAY_PID"

# Wait for any process to exit
wait -n

echo "A consumer process exited. Exiting..."
exit 1
