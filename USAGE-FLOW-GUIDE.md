# Complete Microservices Usage Flow Guide

This guide provides step-by-step instructions on how to use every feature across all microservices with real examples and workflows.

---

## 🎯 Quick Setup (Prerequisites)

### Step 1: Port-Forward All Services

Open 8 terminals and run these commands (one per terminal):

```bash
# Terminal 1
kubectl port-forward svc/frontend 3000:3000 -n retail-mesh &

# Terminal 2
kubectl port-forward svc/order-service 5000:5000 -n retail-mesh &

# Terminal 3
kubectl port-forward svc/inventory-service 5001:5001 -n retail-mesh &

# Terminal 4
kubectl port-forward svc/payment-service 5002:5002 -n retail-mesh &

# Terminal 5
kubectl port-forward svc/notification-service 5003:5003 -n retail-mesh &

# Terminal 6
kubectl port-forward svc/loyalty-service 5004:5004 -n retail-mesh &

# Terminal 7
kubectl port-forward svc/admin-frontend 8000:8000 -n retail-mesh &

# Terminal 8 (Optional - for viewing logs)
kubectl logs -n retail-mesh -f
```

Or run them all in background:
```bash
kubectl port-forward svc/frontend 3000:3000 -n retail-mesh & \
kubectl port-forward svc/order-service 5000:5000 -n retail-mesh & \
kubectl port-forward svc/inventory-service 5001:5001 -n retail-mesh & \
kubectl port-forward svc/payment-service 5002:5002 -n retail-mesh & \
kubectl port-forward svc/notification-service 5003:5003 -n retail-mesh & \
kubectl port-forward svc/loyalty-service 5004:5004 -n retail-mesh & \
kubectl port-forward svc/admin-frontend 8000:8000 -n retail-mesh &
```

---

## 📊 COMPLETE USER JOURNEY (Full End-to-End Flow)

### Scenario: New Customer Places Order and Tracks Rewards

#### **Step 1: Browse Products (Frontend + Inventory)**

**What happens:** Customer visits frontend, sees product catalog, checks inventory.

```bash
# 1a. Open Frontend UI in browser
open http://localhost:3000

# OR get the HTML
curl http://localhost:3000

# 1b. Check Inventory (Backend API)
curl http://localhost:5001/items | jq
```

**Response shows:**
- SKU-001: Laptop ($999.99) - Available: 50
- SKU-002: Mouse ($29.99) - Available: 200
- SKU-003: Keyboard ($79.99) - Available: 150

#### **Step 2: Check Stock for Specific Product**

```bash
# Check if SKU-001 has stock for quantity 1
curl -X POST http://localhost:5001/check-stock \
  -H "Content-Type: application/json" \
  -d '{
    "item_id": "SKU-001",
    "quantity_requested": 1
  }' | jq
```

**Response:**
```json
{
  "available": true,
  "item_id": "SKU-001",
  "item_name": "Laptop",
  "in_stock": 50,
  "requested": 1,
  "price": 999.99,
  "trace_id": null
}
```

#### **Step 3: Create Order (Frontend → Order Service)**

**What happens:** 
- Customer clicks "Buy" on Frontend
- Frontend sends order to Order Service
- Order Service validates inventory
- Order gets stored in PostgreSQL
- Payment is triggered asynchronously

```bash
# Create unique trace ID for this order
TRACE_ID="order-$(date +%s%N | cut -b1-13)"
echo "Trace ID: $TRACE_ID"

# Customer creates order
curl -X POST http://localhost:3000/api/order \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: $TRACE_ID" \
  -d '{
    "item_id": "SKU-001",
    "quantity": 1,
    "customer_id": "CUST-NEW-001",
    "total_price": 999.99
  }' | jq
```

**Response:**
```json
{
  "status": "success",
  "message": "Order placed successfully",
  "order_id": 1,
  "trace_id": "order-1709465xxx"
}
```

#### **Step 4: Verify Order Created (Order Service)**

```bash
# Get the order that was just created
curl http://localhost:5000/api/order/1 \
  -H "x-b3-traceid: $TRACE_ID" | jq
```

**Response:**
```json
{
  "order_id": 1,
  "customer_id": "CUST-NEW-001",
  "item_id": "SKU-001",
  "quantity": 1,
  "unit_price": 999.99,
  "tax": 100.00,
  "total_price": 1099.99,
  "status": "pending",
  "created_at": "2026-03-03T10:00:00Z",
  "trace_id": "order-1709465xxx"
}
```

#### **Step 5: Process Payment (Payment Service)**

**What happens:**
- Payment Service processes the payment asynchronously
- 90% chance of success, 10% chance of failure
- Notification Service is called with result

```bash
# Process payment
curl -X POST http://localhost:5002/process-payment \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: $TRACE_ID" \
  -d '{
    "order_id": 1,
    "amount": 1099.99,
    "customer_id": "CUST-NEW-001"
  }' | jq
```

**Response (Success - 90% of time):**
```json
{
  "status": "success",
  "order_id": 1,
  "transaction_id": "TXN-a1b2c3d4e5f6",
  "amount": 1099.99,
  "currency": "USD",
  "timestamp": "2026-03-03T10:00:30Z",
  "message": "Payment processed successfully",
  "trace_id": "order-1709465xxx"
}
```

**Response (Failure - 10% of time):**
```json
{
  "status": "failed",
  "order_id": 1,
  "transaction_id": "TXN-x1y2z3a4b5c6",
  "amount": 1099.99,
  "error": "Bank declined payment",
  "error_code": "INSUFFICIENT_FUNDS",
  "message": "Payment processing failed",
  "trace_id": "order-1709465xxx"
}
```

#### **Step 6: Check Order Status After Payment**

```bash
# Payment updates order status to "completed" or "failed"
curl http://localhost:5000/api/order/1 | jq '.status'

# Response: "completed" (if payment succeeded)
```

#### **Step 7: Calculate & View Loyalty Points**

**What happens:**
- Points earned: floor(order_amount × random(1.0-1.1))
- Points stored per customer
- Customer accumulates tier status

```bash
# Calculate loyalty points for this order
curl -X POST http://localhost:5004/calculate-points \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: $TRACE_ID" \
  -d '{
    "customer_id": "CUST-NEW-001",
    "order_amount": 1099.99
  }' | jq
```

**Response:**
```json
{
  "customer_id": "CUST-NEW-001",
  "order_amount": 1099.99,
  "points_earned": 1199,
  "multiplier": 1.090,
  "timestamp": "2026-03-03T10:00:35Z",
  "total_balance": 1199,
  "trace_id": "order-1709465xxx"
}
```

#### **Step 8: Check Customer Loyalty Balance**

```bash
# Get total points and tier for customer
curl http://localhost:5004/customer/CUST-NEW-001/points | jq
```

**Response:**
```json
{
  "customer_id": "CUST-NEW-001",
  "total_points": 1199,
  "tier": "Silver",
  "last_activity": "2026-03-03T10:00:35Z",
  "transactions": [
    {
      "order_id": 1,
      "amount": 1099.99,
      "points_earned": 1199,
      "earned_at": "2026-03-03T10:00:35Z"
    }
  ]
}
```

#### **Step 9: View Complete Order History**

```bash
# Get all orders in system
curl "http://localhost:5000/api/orders?limit=50&offset=0" | jq

# Filter by specific customer (using grep)
curl "http://localhost:5000/api/orders?limit=50&offset=0" | jq '.orders[] | select(.customer_id=="CUST-NEW-001")'
```

#### **Step 10: Monitor Everything in Admin Dashboard**

```bash
# Open Admin Dashboard
open http://localhost:8000

# Or API endpoint
curl http://localhost:8000/api/dashboard | jq
```

**Dashboard shows:**
- Total Orders: 1
- Total Products: 3
- Low Stock Items: 0
- Services Health: 4/4 healthy
- Recent Orders table with your order
- Inventory table with updated stock
- Services Health table

---

## 🏪 INDIVIDUAL SERVICE USE CASES

### Scenario 1: Inventory Manager Checks Stock

```bash
# 1. List all products
curl http://localhost:5001/items | jq

# 2. Check specific product details
curl http://localhost:5001/items/SKU-002 | jq

# 3. Check if product is in stock with quantity
curl -X POST http://localhost:5001/check-stock \
  -H "Content-Type: application/json" \
  -d '{
    "item_id": "SKU-002",
    "quantity_requested": 100
  }' | jq
```

---

### Scenario 2: Order Manager Views All Orders

```bash
# 1. Get all orders
curl http://localhost:5000/api/orders | jq

# 2. Get specific order
curl http://localhost:5000/api/order/1 | jq

# 3. Create new order (via API, not UI)
curl -X POST http://localhost:5000/api/order \
  -H "Content-Type: application/json" \
  -d '{
    "item_id": "SKU-003",
    "quantity": 5,
    "customer_id": "BULK-CUST",
    "total_price": 399.95
  }' | jq
```

---

### Scenario 3: Loyalty Manager Tracks Customer Rewards

```bash
# 1. Calculate points for multiple orders
curl -X POST http://localhost:5004/calculate-points \
  -d '{"customer_id":"CUST-001","order_amount":500}' | jq

curl -X POST http://localhost:5004/calculate-points \
  -d '{"customer_id":"CUST-001","order_amount":750}' | jq

curl -X POST http://localhost:5004/calculate-points \
  -d '{"customer_id":"CUST-001","order_amount":1200}' | jq

# 2. View customer total balance
curl http://localhost:5004/customer/CUST-001/points | jq

# 3. Redeem points for discount
curl -X POST http://localhost:5004/redeem-points \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "CUST-001",
    "points_to_redeem": 500
  }' | jq
```

**Response:**
```json
{
  "customer_id": "CUST-001",
  "points_redeemed": 500,
  "discount_value": 5.00,
  "remaining_points": 2450,
  "redemption_id": "REDEEM-abc123"
}
```

---

### Scenario 4: Payment Officer Processes & Tracks Payments

```bash
# 1. Process payment
curl -X POST http://localhost:5002/process-payment \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": 1,
    "amount": 1099.99,
    "customer_id": "CUST-PAY"
  }' | jq

# Save transaction ID from response
# TXN-a1b2c3d4e5f6

# 2. Check transaction status
curl http://localhost:5002/transaction/TXN-a1b2c3d4e5f6 | jq

# 3. Run multiple payment tests to see success/failure rates
for i in {1..10}; do
  echo "=== Payment Attempt $i ==="
  curl -s -X POST http://localhost:5002/process-payment \
    -H "Content-Type: application/json" \
    -d "{
      \"order_id\": $i,
      \"amount\": 999.99,
      \"customer_id\": \"CUST-BATCH\"
    }" | jq '.status'
done

# Shows mix of "success" (90%) and "failed" (10%)
```

---

## 🔄 DATA FLOW VISUALIZATION

### Order Creation Flow

```
Customer
   ↓
Frontend (3000)
   │ - Displays product catalog
   │ - Session management (Redis)
   └─→ POST /api/order
       │
       Order Service (5000)
       │ - Validates inventory
       │ - Calculates tax
       │ - Stores in PostgreSQL
       └─→ Payment Service (5002) async
           │ - Process payment (0.5-2s delay)
           │ - 90% success, 10% failure
           └─→ Notification Service (3003)
               └─ Log notification

Stock Flow
Inventory Service (5001)
   │ - Check stock (MongoDB)
   │ - Reserve stock
   └─ Release stock

Rewards Flow
Loyalty Service (5004)
   │ - Calculate points (floor(amount × 1.0-1.1))
   │ - Store per customer
   └─ Redeem points
```

---

## 📝 COMMON WORKFLOWS

### Workflow 1: Process New Customer Order (Beginner)

```bash
#!/bin/bash

CUSTOMER="NEW-CUST-$(date +%s)"
TRACE_ID="trace-$(date +%s%N | cut -b1-13)"

# Step 1: Check inventory
echo "=== Step 1: Check Inventory ==="
curl -s -X POST http://localhost:5001/check-stock \
  -H "Content-Type: application/json" \
  -d '{"item_id":"SKU-001","quantity_requested":1}' | jq '.available'

# Step 2: Create order
echo -e "\n=== Step 2: Create Order ==="
ORDER_RESPONSE=$(curl -s -X POST http://localhost:3000/api/order \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: $TRACE_ID" \
  -d "{
    \"item_id\": \"SKU-001\",
    \"quantity\": 1,
    \"customer_id\": \"$CUSTOMER\",
    \"total_price\": 999.99
  }")
echo $ORDER_RESPONSE | jq
ORDER_ID=$(echo $ORDER_RESPONSE | jq '.order_id')

# Step 3: Process payment
echo -e "\n=== Step 3: Process Payment ==="
PAYMENT=$(curl -s -X POST http://localhost:5002/process-payment \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: $TRACE_ID" \
  -d "{
    \"order_id\": $ORDER_ID,
    \"amount\": 1099.99,
    \"customer_id\": \"$CUSTOMER\"
  }")
echo $PAYMENT | jq
PAYMENT_STATUS=$(echo $PAYMENT | jq '.status')

# Step 4: Calculate loyalty points
echo -e "\n=== Step 4: Calculate Loyalty Points ==="
curl -s -X POST http://localhost:5004/calculate-points \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: $TRACE_ID" \
  -d "{
    \"customer_id\": \"$CUSTOMER\",
    \"order_amount\": 1099.99
  }" | jq

# Step 5: Get order status
echo -e "\n=== Step 5: Check Final Order Status ==="
curl -s http://localhost:5000/api/order/$ORDER_ID | jq '{status: .status, total: .total_price}'

# Step 6: View in admin dashboard
echo -e "\n=== Step 6: View in Admin Dashboard ==="
echo "Open: http://localhost:8000"
```

### Workflow 2: Bulk Order Processing (Intermediate)

```bash
#!/bin/bash

# Process 5 orders from different customers
for i in {1..5}; do
  CUSTOMER="BULK-CUST-$i"
  ITEM="SKU-00$((i % 3 + 1))"
  QTY=$((i))
  
  echo "=== Processing Order for $CUSTOMER ==="
  
  # Create order
  curl -s -X POST http://localhost:5000/api/order \
    -H "Content-Type: application/json" \
    -d "{
      \"item_id\": \"$ITEM\",
      \"quantity\": $QTY,
      \"customer_id\": \"$CUSTOMER\",
      \"total_price\": $((100 * i))
    }" | jq '.order_id'
  
  # Add slight delay
  sleep 0.5
done

# View all orders
echo -e "\n=== All Orders ==="
curl -s http://localhost:5000/api/orders | jq '.total'
```

### Workflow 3: Loyalty Program Simulation (Intermediate)

```bash
#!/bin/bash

CUSTOMER="LOYALTY-VIP-001"

# Simulate 3 purchases
PURCHASES=(500 750 1200)

echo "=== Loyalty Program Tracking for $CUSTOMER ==="

for amount in "${PURCHASES[@]}"; do
  echo "Processing $amount purchase..."
  
  # Calculate points
  POINTS=$(curl -s -X POST http://localhost:5004/calculate-points \
    -H "Content-Type: application/json" \
    -d "{
      \"customer_id\": \"$CUSTOMER\",
      \"order_amount\": $amount
    }" | jq '.points_earned')
  
  echo "Points earned: $POINTS"
  sleep 0.5
done

# Check final balance
echo -e "\n=== Final Customer Status ==="
curl -s http://localhost:5004/customer/$CUSTOMER/points | jq '{tier: .tier, total_points: .total_points}'

# Try to redeem points
echo -e "\n=== Redeem 500 Points ==="
curl -s -X POST http://localhost:5004/redeem-points \
  -H "Content-Type: application/json" \
  -d "{
    \"customer_id\": \"$CUSTOMER\",
    \"points_to_redeem\": 500
  }" | jq '{discount_value: .discount_value, remaining_points: .remaining_points}'
```

---

## ✅ VERIFICATION CHECKLIST

After completing flows, verify everything:

```bash
# 1. Check all pods are running
kubectl get pods -n retail-mesh | grep "1/1 Running"

# 2. Check all services are up
kubectl get svc -n retail-mesh

# 3. Test each service health
curl http://localhost:3000/health
curl http://localhost:5000/health
curl http://localhost:5001/health
curl http://localhost:5002/health
curl http://localhost:5003/health
curl http://localhost:5004/health
curl http://localhost:8000/health

# 4. View recent logs with trace ID
TRACEID="your-trace-id-here"
kubectl logs -n retail-mesh -l app=order-service | grep $TRACEID
kubectl logs -n retail-mesh -l app=inventory-service | grep $TRACEID
kubectl logs -n retail-mesh -l app=payment-service | grep $TRACEID

# 5. Check database contents
# PostgreSQL orders
kubectl exec -it postgres-xxx -n retail-mesh -- psql -U postgres -d orders_db -c "SELECT * FROM orders LIMIT 5;"

# MongoDB inventory
kubectl exec -it mongodb-xxx -n retail-mesh -- mongosh retail_db --eval "db.inventory.find().limit(5)"

# Redis sessions
kubectl exec -it redis-xxx -n retail-mesh -- redis-cli KEYS "session:*" | head -10
```

---

## 🎓 LEARNING PATHS

### Beginner Path (1-2 hours)
1. Open Admin Dashboard (http://localhost:8000)
2. Run Workflow 1: Process single order
3. Verify in Admin Dashboard
4. Check logs

### Intermediate Path (2-4 hours)
1. Complete all individual service scenarios
2. Run Workflow 2: Bulk order processing
3. Run Workflow 3: Loyalty program simulation
4. Monitor in Admin Dashboard
5. Check database queries

### Advanced Path (4+ hours)
1. Integrate with Istio (service mesh)
2. Set up Jaeger for distributed tracing
3. Create custom Docker images with modifications
4. Deploy Prometheus metrics collection
5. Create CI/CD pipeline

---

## 🔍 DEBUGGING & MONITORING

### View System Metrics

```bash
# Check resource usage
kubectl top pods -n retail-mesh

# Check service endpoints
kubectl get endpoints -n retail-mesh

# Describe specific service
kubectl describe svc order-service -n retail-mesh
```

### Real-time Monitoring

```bash
# Watch pods in real-time
kubectl get pods -n retail-mesh -w

# Follow logs from all services
kubectl logs -n retail-mesh --all-containers=true -f

# Filter logs by specific service
kubectl logs -n retail-mesh -l app=order-service -f --tail=50
```

### Trace Request Flow

```bash
# Create a trace ID
TRACE_ID="debug-$(date +%s%N | cut -b1-13)"

# Make request with trace ID
curl -X POST http://localhost:5000/api/order \
  -H "x-b3-traceid: $TRACE_ID" \
  -d '...'

# View logs for this request across all services
kubectl logs -n retail-mesh --all-containers=true | grep $TRACE_ID
```

---

## 💡 KEY CONCEPTS

### Trace IDs
Every request can have a unique trace ID (B3 format) that flows through all microservices. Useful for debugging.

### Order Status Lifecycle
```
pending → completed (payment success)
       → failed (payment denied)
```

### Loyalty Points Calculation
```
Points = floor(order_amount × random(1.0-1.1))
Example: $1000 × 1.087 = 1087 points
```

### Service Dependencies
```
Frontend depends on: Redis + Order Service
Order Service depends on: PostgreSQL + Inventory Service + Payment Service
Inventory Service depends on: MongoDB
Payment Service depends on: Notification Service
Loyalty Service depends on: Nothing
Admin Dashboard depends on: All services (for aggregation)
```

---

## 🚀 NEXT STEPS

1. **Complete all workflows** - Try all scenarios above
2. **Automate testing** - Create shell scripts for repetitive flows
3. **Monitor performance** - Use Admin Dashboard for real-time insights
4. **Integrate Istio** - Add service mesh for advanced traffic management
5. **Setup Jaeger** - Visualize distributed traces

Happy testing! 🎉
