# Retail Mesh - System Architecture & Process Flow

## 🏗️ System Overview

The Retail Mesh is a 6-microservice distributed system designed for learning Istio traffic management, Jaeger distributed tracing, and resilience patterns. All services propagate B3 trace headers for end-to-end observability.

---

## 📊 Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                          USER / CLIENT                              │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
                    HTTP Request (Port 3000)
                                 ▼
                    ┌────────────────────────┐
                    │   FRONTEND SERVICE     │
                    │ (Go) Port 3000         │
                    │ ✓ Serves HTML UI       │
                    │ ✓ Session Management   │
                    │ ✓ Redis Integration    │
                    └────────┬───────────────┘
                             │
                             │ HTTP Call to Order Service
                             │ (Port 5000) + B3 Headers
                             ▼
                    ┌────────────────────────┐
                    │   ORDER SERVICE        │
                    │ (Go) Port 5000         │
                    │ ✓ Order Orchestrator   │
                    │ ✓ PostgreSQL Persist   │
                    └──────┬──────┬──────────┘
                           │      │
                 ┌─────────┘      └────────────┐
                 │                             │
     Stock Check Call         Async Payment Call
     (Port 5001)              (Port 5002)
                 │                             │
                 ▼                             ▼
        ┌─────────────────┐        ┌──────────────────┐
        │ INVENTORY SVC   │        │ PAYMENT SERVICE  │
        │ (Python)        │        │ (Go) Port 5002   │
        │ Port 5001       │        │ ✓ Bank Gateway   │
        │ ✓ MongoDB       │        │ ✓ 10% Failure    │
        │ ✓ Stock Check   │        │   Simulation     │
        └─────────────────┘        └────────┬─────────┘
                                            │
                                    Async Notification
                                    (Port 5003)
                                            │
                                            ▼
                                 ┌─────────────────────┐
                                 │ NOTIFICATION SVC    │
                                 │ (Go) Port 5003      │
                                 │ ✓ Event Handler     │
                                 │ ✓ Async Process     │
                                 └─────────────────────┘

                    INDEPENDENT SERVICE:
                    
                    ┌─────────────────────┐
                    │  LOYALTY SERVICE    │
                    │ (Python) Port 5004  │
                    │ ✓ Points Calculator │
                    │ ✓ In-Memory Store   │
                    └─────────────────────┘

───────────────────────────────────────────────────────────────────────

DATABASES:

┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  PostgreSQL  │    │   MongoDB    │    │    Redis     │
│  Port 5432   │    │  Port 27017  │    │  Port 6379   │
│              │    │              │    │              │
│ Orders DB    │    │ Inventory    │    │ Sessions     │
│ (Relational) │    │ (Document)   │    │ (Cache)      │
└──────────────┘    └──────────────┘    └──────────────┘
     ▲                    ▲                    ▲
     │                    │                    │
     └─ Order Service     │─ Inventory Svc     └─ Frontend
                          
──────────────────────────────────────────────────────────────────────
```

---

## 🔄 Complete Request Flow

### **Scenario: User Places an Order**

#### **Step 1: User visits Frontend (http://localhost:3000)**
```
Client Browser
  ↓
GET http://localhost:3000/
  ↓
Frontend Service receives request
  ├─ Extracts/Creates B3 Headers
  ├─ Creates/retrieves Redis Session
  └─ Returns HTML with product catalog
```

#### **Step 2: User clicks "Buy" button**
```
Client Browser
  ↓
POST http://localhost:3000/api/order
  │ Payload: { item_id: "SKU-001", quantity: 1, customer_id: "CUST-123" }
  │ Headers: x-b3-traceid: abc123xyz...
  ↓
Frontend Service receives order request
  ├─ Extracts B3 trace headers
  ├─ Forwards to Order Service: POST http://order-service:5000/place-order
  └─ Passes through ALL B3 headers (traceid, spanid, sampled, etc.)
```

#### **Step 3: Order Service validates & checks inventory**
```
Order Service receives request
  ├─ Logs incoming TraceID: abc123xyz
  ├─ Calls Inventory Service: POST http://inventory-service:5001/check-stock
  │   ├─ Headers include B3 traceid: abc123xyz (SAME TRACE ID!)
  │   ├─ Payload: { item_id: "SKU-001", quantity_requested: 1 }
  │   └─ Receives: { available: true, in_stock: 50 }
  │
  ├─ Stores order in PostgreSQL:
  │   └─ INSERT INTO orders (item_id, quantity, trace_id, created_at) 
  │       VALUES ('SKU-001', 1, 'abc123xyz', NOW())
  │
  └─ Returns response to Frontend with same TraceID
```

#### **Step 4: Payment Processing (Async)**
```
Order Service (after storing order)
  ├─ Calls Payment Service asynchronously
  │   └─ POST http://payment-service:5002/process-payment
  │       ├─ Headers: x-b3-traceid: abc123xyz (PRESERVED!)
  │       ├─ Payload: { order_id: 1, amount: 99.99 }
  │       └─ Does NOT wait for response
  │
Payment Service
  ├─ Simulates bank processing (0.5-2s delay)
  ├─ 90% success, 10% failure
  ├─ Generates transaction ID: TXN-36b2e46f
  │
  └─ Calls Notification Service asynchronously
      └─ POST http://notification-service:5003/send-notification
          ├─ Preserves TraceID: abc123xyz
          ├─ Payload: { order_id: 1, txn_id: "TXN-36b2e46f", status: "success" }
          │
          Notification Service
            └─ Logs notification with TraceID
            └─ (Could integrate with email/SMS here)
```

#### **Step 5: Loyalty Points (Independent)**
```
Can be called independently at any time
  ↓
POST http://localhost:5004/calculate-points
  │ Payload: { customer_id: "CUST-123", order_amount: 99.99 }
  ↓
Loyalty Service
  ├─ Calculates: floor(99.99 * random(1.0-1.1))
  ├─ Example result: 109 points
  ├─ Stores in in-memory dictionary
  └─ Returns: { points_earned: 109, total_balance: 500 }
```

---

## 🌐 Accessing Individual Services

### **1. Frontend Service**

**Purpose:** Web UI and entry point  
**Language:** Go 1.21  
**Location:** http://localhost:3000  
**Port:** 3000  
**Docker Image:** `awezkhan6899/retail-frontend:latest`  
**Source:** `/frontend/main.go`

#### **Port-Forward:**
```bash
kubectl port-forward svc/frontend 3000:3000 -n retail
```

#### **Core Features:**
1. **HTML UI** - Serves a dynamic product listing page
   - Displays 3 sample products (SKU-001, SKU-002, SKU-003)
   - Each product has "Buy" button
   - Shows product name, price, and description
   
2. **Session Management** - Uses Redis for session storage
   - Session ID created on first visit (UUID format)
   - Session expires after 24 hours
   - Stores customer info: `session:{sessionId} → JSON`
   - Session data includes: customer_id, email, created_at

3. **B3 Header Propagation**
   - Extracts incoming trace headers if present
   - Generates new trace ID if not present
   - Forwards all headers to downstream Order Service

4. **Request Handling**
   - HTML page renders product catalog UI
   - Handles form submission
   - Makes HTTP call to Order Service with full trace context

#### **Environment Variables:**
```bash
REDIS_HOST=redis          # Redis hostname
REDIS_PORT=6379          # Redis port
ORDER_SERVICE_HOST=order-service  # Order Service hostname
ORDER_SERVICE_PORT=5000   # Order Service port
PORT=3000                 # Frontend port
```

#### **Endpoints:**
- `GET /` - Serve HTML UI with product list
  ```bash
  curl http://localhost:3000
  ```
  Returns HTML page with product catalog and "Buy" buttons

- `POST /api/order` - Create order (called via HTML form)
  ```bash
  curl -X POST http://localhost:3000/api/order \
    -H "Content-Type: application/json" \
    -H "x-b3-traceid: my-trace-123" \
    -d '{
      "item_id": "SKU-001",
      "quantity": 2,
      "customer_id": "CUST-ABC",
      "total_price": 199.98
    }'
  ```
  **Response:**
  ```json
  {
    "status": "success",
    "message": "Order placed successfully",
    "order_id": 1,
    "trace_id": "my-trace-123"
  }
  ```

- `GET /health` - Health check endpoint
  ```bash
  curl http://localhost:3000/health
  ```
  Returns 200 OK and `"OK"` if service is healthy

#### **Session Storage Example (Redis):**
```
Key: session:550e8400-e29b-41d4-a716-446655440000
Value: {
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "customerId": "CUST-ABC",
  "email": "customer@example.com",
  "createdAt": "2026-03-02T18:30:00Z",
  "lastActivity": "2026-03-02T18:35:00Z"
}
TTL: 86400 seconds (24 hours)
```

#### **Business Logic:**
```go
// Session creation on first request
sessionID := uuid.New().String()
rdb.Set(ctx, "session:"+sessionID, sessionData, 24*time.Hour)

// Order API call
func orderAPIHandler(w http.ResponseWriter, r *http.Request) {
  // 1. Extract/Create trace ID
  traceID := extractTraceID(r.Header)
  
  // 2. Unmarshal order request
  var order OrderRequest
  json.NewDecoder(r.Body).Decode(&order)
  
  // 3. Call Order Service with trace headers
  headers := http.Header{
    "x-b3-traceid": {traceID},
    "x-b3-sampled": {"1"},
    // ... other B3 headers
  }
  
  // 4. Return response with same trace ID
}
```

#### **Dependencies:**
- **Redis** - Session storage (required for startup)
- **Order Service** - Order placement and validation
- **Kubernetes Network** - Service discovery via DNS

#### **Health Check Details:**
```go
func healthHandler(w http.ResponseWriter, r *http.Request) {
  // Check redis connection
  err := rdb.Ping(ctx).Err()
  if err != nil {
    w.WriteHeader(http.StatusServiceUnavailable)
    return
  }
  w.WriteHeader(http.StatusOK)
  w.Write([]byte("OK"))
}
```

#### **Troubleshooting:**
- **CrashLoopBackOff:** Check Redis connectivity
  ```bash
  kubectl logs frontend-xxx -n retail
  # Look for: "Connected to Redis" message
  ```
- **Session not persisting:** Verify Redis is running
  ```bash
  kubectl get pods -n retail | grep redis
  ```
- **Orders not being created:** Check Order Service connectivity
  ```bash
  kubectl logs frontend-xxx -n retail | grep "order-service"
  ```

---

### **2. Order Service**

**Purpose:** Core order processing engine  
**Language:** Go 1.21  
**Location:** http://localhost:5000  
**Port:** 5000  
**Docker Image:** `awezkhan6899/retail-order:latest`  
**Source:** `/order-service/main.go`

#### **Port-Forward:**
```bash
kubectl port-forward svc/order-service 5000:5000 -n retail
```

#### **Core Features:**
1. **Order Creation & Validation**
   - Validates item exists in inventory
   - Checks stock availability
   - Calculates tax (10% of subtotal)
   - Generates order ID (auto-increment)

2. **Database Schema (PostgreSQL)**
   ```sql
   CREATE TABLE IF NOT EXISTS orders (
     id SERIAL PRIMARY KEY,
     customer_id VARCHAR(50),
     item_id VARCHAR(50),
     quantity INT,
     unit_price DECIMAL(10,2),
     tax DECIMAL(10,2),
     total_price DECIMAL(10,2),
     status VARCHAR(20) DEFAULT 'pending',
     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
     updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
   );
   
   -- Sample data
   INSERT INTO orders (customer_id, item_id, quantity, unit_price, tax, total_price, status)
   VALUES ('CUST-ABC', 'SKU-001', 2, 99.99, 20.00, 219.98, 'pending');
   ```

3. **Service Integration**
   - Calls Inventory Service to get product details and reserve stock
   - Triggers Payment Service asynchronously
   - Updates order status based on payment outcome
   - Preserves B3 trace ID across all calls

4. **B3 Header Propagation**
   - Extracts trace headers from Frontend
   - Adds server span information
   - Forwards complete trace context to downstream services

#### **Environment Variables:**
```bash
DB_HOST=postgres         # PostgreSQL hostname
DB_PORT=5432            # PostgreSQL port
DB_USER=postgres        # Database user
DB_PASSWORD=password    # Database password
DB_NAME=orders_db       # Database name
INVENTORY_SERVICE_HOST=inventory-service  # Inventory hostname
INVENTORY_SERVICE_PORT=5001  # Inventory port
PAYMENT_SERVICE_HOST=payment-service  # Payment hostname
PAYMENT_SERVICE_PORT=5002  # Payment port
PORT=5000               # Order Service port
```

#### **Endpoints:**

**POST /api/order** - Create new order
```bash
curl -X POST http://localhost:5000/api/order \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: my-trace-123" \
  -d '{
    "item_id": "SKU-001",
    "quantity": 2,
    "customer_id": "CUST-ABC",
    "total_price": 199.98
  }'
```
**Request Body:**
```json
{
  "item_id": "SKU-001",       # Product ID
  "quantity": 2,              # Quantity ordered
  "customer_id": "CUST-ABC",  # Customer identifier
  "total_price": 199.98       # Pre-calculated total price
}
```
**Response (201 Created):**
```json
{
  "order_id": 1,
  "customer_id": "CUST-ABC",
  "item_id": "SKU-001",
  "quantity": 2,
  "unit_price": 99.99,
  "tax": 20.00,
  "total_price": 219.98,
  "status": "pending",
  "created_at": "2026-03-02T18:35:00Z",
  "trace_id": "my-trace-123"
}
```
**Response (400 Bad Request - Stock unavailable):**
```json
{
  "error": "Insufficient inventory for SKU-001",
  "requested": 2,
  "available": 1,
  "trace_id": "my-trace-123"
}
```

**GET /api/order/{order_id}** - Retrieve order details
```bash
curl http://localhost:5000/api/order/1 \
  -H "x-b3-traceid: my-trace-123"
```
**Response (200 OK):**
```json
{
  "order_id": 1,
  "customer_id": "CUST-ABC",
  "item_id": "SKU-001",
  "quantity": 2,
  "status": "completed",
  "unit_price": 99.99,
  "tax": 20.00,
  "total_price": 219.98,
  "created_at": "2026-03-02T18:35:00Z",
  "updated_at": "2026-03-02T18:35:30Z",
  "trace_id": "my-trace-123"
}
```

**GET /api/orders** - List all orders with pagination
```bash
curl "http://localhost:5000/api/orders?limit=10&offset=0" \
  -H "x-b3-traceid: my-trace-123"
```
**Response:**
```json
{
  "orders": [
    {
      "order_id": 1,
      "customer_id": "CUST-ABC",
      "item_id": "SKU-001",
      "status": "completed",
      "total_price": 219.98,
      "created_at": "2026-03-02T18:35:00Z"
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

**GET /health** - Health check
```bash
curl http://localhost:5000/health
```
Returns 200 OK and database connection status

#### **Business Logic Flow:**

```go
// Order creation workflow
func createOrder(order OrderRequest) error {
  // 1. Validate inventory
  inventory := callInventoryService(order.ItemID)
  if inventory.Available < order.Quantity {
    return errors.New("insufficient inventory")
  }
  
  // 2. Calculate prices
  unitPrice := inventory.Price
  subtotal := unitPrice * float64(order.Quantity)
  tax := subtotal * 0.10  // 10% tax
  totalPrice := subtotal + tax
  
  // 3. Save to PostgreSQL
  err := insertOrder(
    order.CustomerID,
    order.ItemID,
    order.Quantity,
    unitPrice,
    tax,
    totalPrice,
    "pending",
  )
  
  // 4. Reserve inventory (reduce stock)
  callInventoryService("reserve", order.ItemID, order.Quantity)
  
  // 5. Trigger payment async
  asyncPaymentCall(orderID, totalPrice)
  
  // 6. Return with trace ID
  return nil
}

// Order status update on payment completion
func updateOrderStatus(orderID int, paymentStatus string) {
  status := "failed"
  if paymentStatus == "success" {
    status = "completed"
  }
  updateDatabase("UPDATE orders SET status = ? WHERE id = ?", status, orderID)
}
```

#### **Database Queries:**

**Create new order:**
```sql
INSERT INTO orders (customer_id, item_id, quantity, unit_price, tax, total_price, status)
VALUES ('CUST-ABC', 'SKU-001', 2, 99.99, 20.00, 219.98, 'pending');
```

**Get order by ID:**
```sql
SELECT * FROM orders WHERE id = 1;
```

**Update status:**
```sql
UPDATE orders SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id = 1;
```

**Get all orders for customer:**
```sql
SELECT * FROM orders WHERE customer_id = 'CUST-ABC' ORDER BY created_at DESC;
```

#### **Dependencies:**
- **PostgreSQL** - Order data storage (required for startup)
- **Inventory Service** - Stock availability check
- **Payment Service** - Async payment processing
- **Kubernetes Network** - Service discovery

#### **Error Handling:**

| Error | Cause | Resolution |
|-------|-------|-----------|
| 500 Database Connection Error | PostgreSQL unreachable | Check `DB_HOST`, `DB_PORT`, credentials |
| 400 Insufficient Inventory | Stock not available | Reduce quantity or wait for re-stock |
| 503 Inventory Service Unavailable | Inventory Service down | Check inventory-service pod status |
| 504 Payment Timeout | Payment service slow | Check payment-service logs |

#### **Logging Pattern:**
```
[ORDER] Creating order for CUST-ABC, qty=2, trace_id=my-trace-123
[ORDER] Validating inventory for SKU-001...
[ORDER] Reserved 2 units from inventory
[ORDER] Order ID: 1, saved to database
[ORDER] Sending to payment service (async)
[ORDER] Order creation completed in 234ms
```

#### **Troubleshooting:**
- **Connection refused to postgres:** Verify DB credentials and postgres pod is running
  ```bash
  kubectl get pods -n retail | grep postgres
  kubectl logs postgres-xxx -n retail
  ```
- **Cannot call inventory service:** Check network connectivity
  ```bash
  kubectl exec order-service-xxx -n retail -- curl http://inventory-service:5001/health
  ```
- **Orders stuck in pending:** Check payment service logs
  ```bash
  kubectl logs payment-service-xxx -n retail | grep "order_id"
  ```

---

### **3. Inventory Service**

**Purpose:** Product catalog and stock management  
**Language:** Python 3.11 with FastAPI  
**Location:** http://localhost:5001  
**Port:** 5001  
**Docker Image:** `awezkhan6899/retail-inventory:latest`  
**Source:** `/inventory-service/app.py`

#### **Port-Forward:**
```bash
kubectl port-forward svc/inventory-service 5001:5001 -n retail
```

#### **Core Features:**
1. **Product Catalog Management**
   - Stores products with name, price, description
   - Manages stock quantities per item
   - Tracks product creation timestamps
   - Supports real-time stock updates

2. **Stock Management Operations**
   - Check stock availability
   - Reserve stock when order is placed
   - Release stock if order is cancelled
   - Track reserved vs available inventory

3. **Database Schema (MongoDB)**
   ```javascript
   // MongoDB collection: inventory
   db.inventory.insertMany([
     {
       "_id": "SKU-001",
       "name": "Laptop",
       "description": "High-performance laptop for development",
       "price": 999.99,
       "quantity": 50,
       "reserved": 0,
       "category": "Electronics",
       "sku_code": "SKU-001",
       "created_at": ISODate("2026-03-01T00:00:00Z"),
       "updated_at": ISODate("2026-03-01T00:00:00Z")
     },
     {
       "_id": "SKU-002",
       "name": "Mouse",
       "description": "Wireless mouse with USB receiver",
       "price": 29.99,
       "quantity": 200,
       "reserved": 0,
       "category": "Accessories",
       "sku_code": "SKU-002",
       "created_at": ISODate("2026-03-01T00:00:00Z"),
       "updated_at": ISODate("2026-03-01T00:00:00Z")
     },
     {
       "_id": "SKU-003",
       "name": "Keyboard",
       "description": "Mechanical keyboard with RGB lighting",
       "price": 79.99,
       "quantity": 150,
       "reserved": 0,
       "category": "Accessories",
       "sku_code": "SKU-003",
       "created_at": ISODate("2026-03-01T00:00:00Z"),
       "updated_at": ISODate("2026-03-01T00:00:00Z")
     }
   ]);
   ```

4. **B3 Header Integration**
   - FastAPI middleware extracts B3 headers
   - Logs include trace_id and span_id
   - Passes trace context to dependent services

#### **Environment Variables:**
```bash
MONGO_HOST=mongodb       # MongoDB hostname
MONGO_PORT=27017        # MongoDB port
MONGO_DBNAME=retail_db  # Database name
PORT=5001               # Inventory Service port
```

#### **Endpoints:**

**POST /check-stock** - Verify product availability
```bash
curl -X POST http://localhost:5001/check-stock \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: inventory-trace-789" \
  -d '{
    "item_id": "SKU-001",
    "quantity_requested": 5
  }'
```
**Request Body:**
```json
{
  "item_id": "SKU-001",
  "quantity_requested": 5
}
```
**Response (200 OK - Available):**
```json
{
  "available": true,
  "item_id": "SKU-001",
  "item_name": "Laptop",
  "in_stock": 50,
  "requested": 5,
  "price": 999.99,
  "trace_id": "inventory-trace-789"
}
```
**Response (400 - Insufficient Stock):**
```json
{
  "available": false,
  "item_id": "SKU-001",
  "requested": 5,
  "in_stock": 2,
  "error": "Insufficient inventory",
  "trace_id": "inventory-trace-789"
}
```

**GET /items** - List all products
```bash
curl http://localhost:5001/items \
  -H "x-b3-traceid: inventory-trace-001"
```
**Response (200 OK):**
```json
{
  "items": [
    {
      "item_id": "SKU-001",
      "name": "Laptop",
      "description": "High-performance laptop for development",
      "category": "Electronics",
      "quantity": 50,
      "reserved": 0,
      "price": 999.99,
      "created_at": "2026-03-01T00:00:00Z"
    },
    {
      "item_id": "SKU-002",
      "name": "Mouse",
      "description": "Wireless mouse with USB receiver",
      "category": "Accessories",
      "quantity": 200,
      "reserved": 0,
      "price": 29.99,
      "created_at": "2026-03-01T00:00:00Z"
    },
    {
      "item_id": "SKU-003",
      "name": "Keyboard",
      "description": "Mechanical keyboard with RGB lighting",
      "category": "Accessories",
      "quantity": 150,
      "reserved": 0,
      "price": 79.99,
      "created_at": "2026-03-01T00:00:00Z"
    }
  ],
  "total": 3,
  "trace_id": "inventory-trace-001"
}
```

**GET /items/{item_id}** - Get specific product details
```bash
curl http://localhost:5001/items/SKU-001 \
  -H "x-b3-traceid: inventory-trace-002"
```
**Response (200 OK):**
```json
{
  "item_id": "SKU-001",
  "name": "Laptop",
  "description": "High-performance laptop for development",
  "category": "Electronics",
  "quantity": 50,
  "reserved": 2,
  "available": 48,
  "price": 999.99,
  "created_at": "2026-03-01T00:00:00Z",
  "updated_at": "2026-03-02T12:34:56Z",
  "trace_id": "inventory-trace-002"
}
```

**POST /reserve-stock** - Reserve inventory (called after order creation)
```bash
curl -X POST http://localhost:5001/reserve-stock \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: inventory-trace-003" \
  -d '{
    "item_id": "SKU-001",
    "quantity": 2,
    "order_id": 1
  }'
```
**Response (200 OK):**
```json
{
  "status": "success",
  "item_id": "SKU-001",
  "order_id": 1,
  "reserved": 2,
  "remaining_available": 48,
  "trace_id": "inventory-trace-003"
}
```

**POST /release-stock** - Release reserved inventory (on order cancellation)
```bash
curl -X POST http://localhost:5001/release-stock \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: inventory-trace-004" \
  -d '{
    "item_id": "SKU-001",
    "quantity": 2,
    "order_id": 1
  }'
```
**Response (200 OK):**
```json
{
  "status": "success",
  "item_id": "SKU-001",
  "order_id": 1,
  "released": 2,
  "available": 50,
  "trace_id": "inventory-trace-004"
}
```

**GET /health** - Health check
```bash
curl http://localhost:5001/health
```
Returns 200 OK and MongoDB connection status

#### **Business Logic (FastAPI/Python):**

```python
from fastapi import FastAPI, Header, HTTPException
from pymongo import MongoClient
from typing import Optional

app = FastAPI()
mongo_client = MongoClient(f"mongodb://{MONGO_HOST}:{MONGO_PORT}/")
db = mongo_client[MONGO_DBNAME]
inventory_collection = db["inventory"]

@app.post("/check-stock")
async def check_stock(item_id: str, quantity_requested: int, 
                      x_b3_traceid: Optional[str] = Header(None)):
    """Check if item is in stock"""
    item = inventory_collection.find_one({"_id": item_id})
    
    if not item:
        raise HTTPException(status_code=404, detail="Item not found")
    
    available = item["quantity"] - item.get("reserved", 0)
    
    return {
        "available": available >= quantity_requested,
        "item_id": item_id,
        "in_stock": available,
        "requested": quantity_requested,
        "price": item["price"],
        "trace_id": x_b3_traceid
    }

@app.post("/reserve-stock")
async def reserve_stock(item_id: str, quantity: int, order_id: int,
                        x_b3_traceid: Optional[str] = Header(None)):
    """Reserve inventory for order"""
    result = inventory_collection.update_one(
        {"_id": item_id},
        {"$inc": {"reserved": quantity}}
    )
    
    available = inventory_collection.find_one(
        {"_id": item_id}
    )["quantity"] - quantity
    
    return {
        "status": "success",
        "item_id": item_id,
        "order_id": order_id,
        "reserved": quantity,
        "remaining_available": available,
        "trace_id": x_b3_traceid
    }
```

#### **MongoDB Queries:**

**Find all inventory items:**
```javascript
db.inventory.find().pretty()
```

**Check stock for specific item:**
```javascript
db.inventory.findOne({ "_id": "SKU-001" })
```

**Update product price:**
```javascript
db.inventory.updateOne(
  { "_id": "SKU-001" },
  { $set: { "price": 899.99, "updated_at": new Date() } }
)
```

**Find items in category:**
```javascript
db.inventory.find({ "category": "Electronics" }).pretty()
```

**Reserve stock (increment reserved counter):**
```javascript
db.inventory.updateOne(
  { "_id": "SKU-001" },
  { $inc: { "reserved": 2 }, $set: { "updated_at": new Date() } }
)
```

#### **Dependencies:**
- **MongoDB** - Inventory data storage (required for startup)
- **Kubernetes Network** - Service discovery for Order Service calls

#### **Error Handling:**

| Error | Cause | Solution |
|-------|-------|----------|
| 500 MongoDB Connection Error | MongoDB unreachable | Check `MONGO_HOST`, `MONGO_PORT` |
| 404 Item Not Found | Invalid SKU | Use valid SKU: SKU-001, SKU-002, SKU-003 |
| 400 Insufficient Stock | Quantity exceeds available | Reduce requested quantity or wait for restock |
| 503 Service Unavailable | Startup timeout | Wait for MongoDB pod to be ready |

#### **Logging Pattern:**
```
[INVENTORY] Received check-stock request: SKU-001, qty=5, trace_id=inventory-trace-789
[INVENTORY] Found item: Laptop, available=50
[INVENTORY] Stock check passed, returning availability
[INVENTORY] Operation completed in 45ms
```

#### **Troubleshooting:**
- **Cannot connect to MongoDB:**
  ```bash
  kubectl exec inventory-service-xxx -n retail -- curl http://mongodb:27017
  ```
- **Missing inventory items:**
  ```bash
  kubectl exec mongodb-xxx -n retail -- mongosh retail_db --eval "db.inventory.find()"
  ```
- **Stock quantities incorrect:**
  ```bash
  # Reset inventory to initial state
  kubectl exec mongodb-xxx -n retail -- mongosh retail_db --eval "db.inventory.updateMany({}, {\$set: {reserved: 0}})"
  ```

---

### **4. Payment Service**

**Purpose:** Payment processing with bank simulation and failure scenarios  
**Language:** Go 1.21  
**Location:** http://localhost:5002  
**Port:** 5002  
**Docker Image:** `awezkhan6899/retail-payment:latest`  
**Source:** `/payment-service/main.go`

#### **Port-Forward:**
```bash
kubectl port-forward svc/payment-service 5002:5002 -n retail
```

#### **Core Features:**
1. **Bank Gateway Simulation**
   - Simulates real bank processing with 0.5-2 second latency
   - **10% random failure rate** for chaos testing
   - Generates unique transaction IDs (TXN-xxxxx)
   - Tracks transaction status (success/failed)

2. **Async Processing**
   - Receives async calls from Order Service
   - Does NOT block Order Service response
   - Asynchronously calls Notification Service after payment (success or failure)
   - Preserves B3 trace ID throughout async flow

3. **Error Simulation**
   - 10% chance of payment failure
   - Returns appropriate error codes (400, 500)
   - Notifies Notification Service of failure
   - Enables testing of retry mechanisms and error handling

4. **B3 Trace Propagation**
   - Extracts trace headers from Order Service
   - Logs all operations with trace_id
   - Forwards to Notification Service with same trace_id

#### **Environment Variables:**
```bash
NOTIFICATION_SERVICE_HOST=notification-service  # Notification Service hostname
NOTIFICATION_SERVICE_PORT=5003  # Notification Service port
PORT=5002               # Payment Service port
PAYMENT_FAILURE_RATE=0.1  # 10% failure rate
PAYMENT_MIN_LATENCY=500    # Minimum 0.5 second latency in ms
PAYMENT_MAX_LATENCY=2000   # Maximum 2 second latency in ms
```

#### **Endpoints:**

**POST /process-payment** - Process payment (async from Order Service)
```bash
curl -X POST http://localhost:5002/process-payment \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: payment-trace-101" \
  -d '{
    "order_id": 1,
    "amount": 999.99,
    "customer_id": "CUST-ABC"
  }'
```
**Request Body:**
```json
{
  "order_id": 1,
  "amount": 999.99,
  "customer_id": "CUST-ABC",
  "currency": "USD"  # Optional
}
```
**Response (200 OK - Success 90% of the time):**
```json
{
  "status": "success",
  "order_id": 1,
  "transaction_id": "TXN-a1b2c3d4e5f6",
  "amount": 999.99,
  "currency": "USD",
  "timestamp": "2026-03-02T18:35:30Z",
  "message": "Payment processed successfully",
  "trace_id": "payment-trace-101"
}
```
**Response (500 - Failure 10% of the time):**
```json
{
  "status": "failed",
  "order_id": 1,
  "transaction_id": "TXN-x1y2z3a4b5c6",
  "amount": 999.99,
  "error": "Bank declined payment",
  "error_code": "INSUFFICIENT_FUNDS",
  "message": "Payment processing failed",
  "trace_id": "payment-trace-101"
}
```

**GET /transaction/{transaction_id}** - Query transaction status
```bash
curl http://localhost:5002/transaction/TXN-a1b2c3d4e5f6 \
  -H "x-b3-traceid: payment-trace-102"
```
**Response (200 OK):**
```json
{
  "transaction_id": "TXN-a1b2c3d4e5f6",
  "order_id": 1,
  "amount": 999.99,
  "status": "success",
  "currency": "USD",
  "processed_at": "2026-03-02T18:35:30Z",
  "notification_sent": true,
  "trace_id": "payment-trace-102"
}
```

**GET /health** - Health check
```bash
curl http://localhost:5002/health
```
Returns 200 OK if service is healthy

#### **Business Logic (Go):**

```go
package main

import (
  "math"
  "math/rand"
  "time"
)

// PaymentRequest represents incoming payment request
type PaymentRequest struct {
  OrderID    int     `json:"order_id"`
  Amount     float64 `json:"amount"`
  CustomerID string  `json:"customer_id"`
  Currency   string  `json:"currency"`
}

// PaymentResponse represents payment result
type PaymentResponse struct {
  Status        string    `json:"status"`
  OrderID       int       `json:"order_id"`
  TransactionID string    `json:"transaction_id"`
  Amount        float64   `json:"amount"`
  Error         *string   `json:"error,omitempty"`
  Timestamp     time.Time `json:"timestamp"`
  TraceID       string    `json:"trace_id"`
}

// Process payment with 10% failure rate and random latency
func processPayment(req PaymentRequest, traceID string) PaymentResponse {
  // Add random latency (0.5 - 2 seconds)
  latency := time.Duration(rand.Intn(1500)+500) * time.Millisecond
  time.Sleep(latency)
  
  // 10% failure rate
  failureCode := rand.Float64()
  if failureCode < 0.10 {
    return PaymentResponse{
      Status:        "failed",
      OrderID:       req.OrderID,
      TransactionID: generateTransactionID(),
      Amount:        req.Amount,
      Error:         stringPtr("Bank declined payment"),
      Timestamp:     time.Now(),
      TraceID:       traceID,
    }
  }
  
  // Success case
  return PaymentResponse{
    Status:        "success",
    OrderID:       req.OrderID,
    TransactionID: generateTransactionID(),
    Amount:        req.Amount,
    Timestamp:     time.Now(),
    TraceID:       traceID,
  }
}

// Generate unique transaction ID
func generateTransactionID() string {
  return fmt.Sprintf("TXN-%s", uuid.New().String()[:12])
}

// After payment, call Notification Service asynchronously
func callNotificationServiceAsync(response PaymentResponse, traceID string) {
  go func() {
    client := &http.Client{Timeout: 5 * time.Second}
    
    payload := map[string]interface{}{
      "order_id":       response.OrderID,
      "transaction_id": response.TransactionID,
      "amount":         response.Amount,
      "status":         response.Status,
    }
    
    req, _ := http.NewRequest("POST", 
      fmt.Sprintf("http://%s:%s/send-notification",
        os.Getenv("NOTIFICATION_SERVICE_HOST"),
        os.Getenv("NOTIFICATION_SERVICE_PORT")),
      // ... pass trace headers
    )
    req.Header.Set("x-b3-traceid", traceID)
    
    client.Do(req)
  }()
}
```

#### **Transaction Flow Diagram:**
```
Order Service (async)
  ↓
Payment Service receives POST /process-payment
  ├─ Extract traceID
  ├─ Add random latency (0.5-2s)
  ├─ Dice roll for failure (10%)
  │  ├─ 90% → Success
  │  │  └─ Generate TXN-xxxxx
  │  └─ 10% → Failure
  │     └─ Generate TXN-xxxxx with error
  │
  └─ Call Notification Service async (preserve traceID)
      ↓
    Notification Service
```

#### **Response Codes:**

| Code | Meaning | Details |
|------|---------|---------|
| 200 | Success | Payment processed, transaction_id generated |
| 500 | Failure | Bank declined, error_code provided |
| 400 | Invalid Request | Missing required fields |
| 503 | Service Unavailable | Cannot reach Notification Service |

#### **Transaction ID Format:**
```
TXN-a1b2c3d4e5f6
     └─ UUID (first 12 chars)
```

#### **Latency Simulation:**
```
Min: 500ms (0.5 seconds)
Max: 2000ms (2.0 seconds)
Random between min and max to simulate real bank processing
```

#### **Failure Scenarios for Testing:**
```
Scenario 1 (10% chance): "Bank declined payment"
  → Status: failed
  → Order remains pending
  → Notification sent to customer

Scenario 2 (90% chance): "Payment successful"
  → Status: success
  → Transaction complete
  → Notification sent with success message
```

#### **Dependencies:**
- **Notification Service** - Async notification after payment (required for full flow)
- **Kubernetes Network** - Service discovery

#### **Error Handling:**

| Error | Cause | Resolution |
|-------|-------|-----------|
| 500 Service Unavailable | Notification Service down | Check notification-service pod |
| 400 Bad Request | Missing order_id or amount | Verify request payload |
| 503 Timeout | Payment takes > 5s | Check service latency |

#### **Logging Pattern:**
```
[PAYMENT] Received payment request: order_id=1, amount=999.99, trace_id=payment-trace-101
[PAYMENT] Adding simulated latency: 1234ms
[PAYMENT] Testing failure rate (10%)...
[PAYMENT] Payment SUCCESS - Transaction: TXN-a1b2c3d4e5f6
[PAYMENT] Calling notification service async...
[PAYMENT] Payment processing completed
```

#### **Troubleshooting:**
- **Payment always fails:** Check failure rate configuration
  ```bash
  kubectl describe pod payment-service-xxx -n retail | grep PAYMENT_FAILURE_RATE
  ```
- **Notification not triggered:** Verify Notification Service is running
  ```bash
  kubectl get pods -n retail | grep notification-service
  kubectl logs notification-service-xxx -n retail
  ```
- **Transactions not tracked:** Check logging/storage in pod logs
  ```bash
  kubectl logs payment-service-xxx -n retail | grep "TXN-"
  ```

---

### **5. Notification Service**

**Purpose:** Async event handler for order and payment notifications  
**Language:** Go 1.21  
**Location:** http://localhost:5003  
**Port:** 5003  
**Docker Image:** `awezkhan6899/retail-notification:latest`  
**Source:** `/notification-service/main.go`

#### **Port-Forward:**
```bash
kubectl port-forward svc/notification-service 5003:5003 -n retail
```

#### **Core Features:**
1. **Event Handler Pattern**
   - Receives async notifications from Payment Service
   - Does not block Payment Service
   - Logs events for observability
   - Can be extended for email/SMS integration

2. **B3 Trace Continuation**
   - Receives trace headers from Payment Service
   - Maintains same trace_id throughout async flow
   - Logs include trace context for distributed tracing
   - Enables end-to-end observability

3. **Event Processing**
   - Processes payment success/failure events
   - Could trigger email notifications (not implemented in demo)
   - Could trigger SMS alerts (not implemented in demo)
   - Could update customer dashboards in real systems

4. **No Persistent Storage**
   - No database dependency
   - Operates as a pure event sink
   - Logs all events to stdout/console
   - In production, could journal events to message queue

#### **Environment Variables:**
```bash
PORT=5003  # Notification Service port
LOG_LEVEL=INFO  # Logging verbosity
```

#### **Endpoints:**

**POST /send-notification** - Receive async notification (called by Payment Service)
```bash
curl -X POST http://localhost:5003/send-notification \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: notification-trace-202" \
  -d '{
    "order_id": 1,
    "transaction_id": "TXN-a1b2c3d4",
    "customer_id": "CUST-ABC",
    "status": "success",
    "amount": 999.99
  }'
```
**Request Body:**
```json
{
  "order_id": 1,
  "transaction_id": "TXN-a1b2c3d4e5f6",
  "customer_id": "CUST-ABC",
  "status": "success",  # success or failed
  "amount": 999.99,
  "message": "Optional custom message"
}
```
**Response (200 OK):**
```json
{
  "status": "success",
  "message": "Notification processed successfully",
  "order_id": 1,
  "notification_id": "NOTIF-12345",
  "trace_id": "notification-trace-202"
}
```

**GET /notifications/{order_id}** - Retrieve notifications for order
```bash
curl http://localhost:5003/notifications/1 \
  -H "x-b3-traceid: notification-trace-203"
```
**Response (200 OK):**
```json
{
  "order_id": 1,
  "notifications": [
    {
      "notification_id": "NOTIF-12345",
      "type": "payment_success",
      "status": "sent",
      "transaction_id": "TXN-a1b2c3d4",
      "message": "Payment successful for order #1",
      "sent_at": "2026-03-02T18:35:30Z",
      "trace_id": "notification-trace-202"
    }
  ],
  "total": 1
}
```

**GET /health** - Health check
```bash
curl http://localhost:5003/health
```
Returns 200 OK

#### **Business Logic (Go):**

```go
package main

import (
  "fmt"
  "time"
  "net/http"
)

// NotificationRequest represents payment event from Payment Service
type NotificationRequest struct {
  OrderID       int     `json:"order_id"`
  TransactionID string  `json:"transaction_id"`
  CustomerID    string  `json:"customer_id"`
  Status        string  `json:"status"`  // success or failed
  Amount        float64 `json:"amount"`
  Message       string  `json:"message"`
}

// NotificationResponse represents acknowledgment
type NotificationResponse struct {
  Status           string    `json:"status"`
  Message          string    `json:"message"`
  OrderID          int       `json:"order_id"`
  NotificationID   string    `json:"notification_id"`
  TraceID          string    `json:"trace_id"`
  ProcessedAt      time.Time `json:"processed_at"`
}

// In-memory notification log (for demo purposes)
var notificationLog map[int][]NotificationRecord

type NotificationRecord struct {
  NotificationID string
  Type           string      // payment_success, payment_failed
  OrderID        int
  CustomerID     string
  TransactionID  string
  Status         string      // sent, failed, pending
  Message        string
  SentAt         time.Time
  TraceID        string
}

// Handler for incoming notifications
func handleNotification(w http.ResponseWriter, r *http.Request) {
  // 1. Parse request
  var notif NotificationRequest
  json.NewDecoder(r.Body).Decode(&notif)
  
  // Get trace ID
  traceID := r.Header.Get("x-b3-traceid")
  
  // 2. Log notification with trace context
  notificationType := "payment_success"
  if notif.Status == "failed" {
    notificationType = "payment_failed"
  }
  
  fmt.Printf("[NOTIFICATION] Processing %s for order %d, trace_id=%s\n",
    notificationType, notif.OrderID, traceID)
  
  // 3. Create notification record
  record := NotificationRecord{
    NotificationID: generateNotificationID(),
    Type:           notificationType,
    OrderID:        notif.OrderID,
    CustomerID:     notif.CustomerID,
    TransactionID:  notif.TransactionID,
    Status:         "sent",
    Message:        notif.Message,
    SentAt:         time.Now(),
    TraceID:        traceID,
  }
  
  // 4. Store in memory log
  if notificationLog[notif.OrderID] == nil {
    notificationLog[notif.OrderID] = []NotificationRecord{}
  }
  notificationLog[notif.OrderID] = append(notificationLog[notif.OrderID], record)
  
  // 5. Could send email/SMS here (not implemented in demo)
  // sendEmail(notif.CustomerID, generateEmailMessage(notif))
  // sendSMS(notif.CustomerID, generateSMSMessage(notif))
  
  // 6. Return acknowledgment
  response := NotificationResponse{
    Status:         "success",
    Message:        "Notification processed successfully",
    OrderID:        notif.OrderID,
    NotificationID: record.NotificationID,
    TraceID:        traceID,
    ProcessedAt:    time.Now(),
  }
  
  w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(response)
  
  fmt.Printf("[NOTIFICATION] Sent notification %s for order %d\n",
    record.NotificationID, notif.OrderID)
}

// Generate unique notification ID
func generateNotificationID() string {
  return fmt.Sprintf("NOTIF-%s", uuid.New().String()[:12])
}

// Generate email message (example)
func generateEmailMessage(notif NotificationRequest) string {
  if notif.Status == "success" {
    return fmt.Sprintf(
      "Dear Customer,\n\nYour payment of $%.2f has been processed successfully.\n"+
        "Order: #%d\nTransaction: %s\n\nThank you!",
      notif.Amount, notif.OrderID, notif.TransactionID)
  }
  return fmt.Sprintf(
    "Dear Customer,\n\nWe encountered an issue processing your payment for order #%d.\n"+
      "Please try again or contact support.\n\nThank you!",
    notif.OrderID)
}
```

#### **Event Flow Diagram:**
```
Payment Service
  │
  └─ POST /send-notification (async, with B3 headers)
      ↓
    Notification Service receives event
      │
      ├─ Extract trace_id
      ├─ Parse payload
      ├─ Log to console/file
      ├─ Could send email/SMS (not in demo)
      └─ Return 200 OK (non-blocking)
```

#### **Notification Types:**

| Type | Trigger | Message |
|------|---------|---------|
| `payment_success` | Payment succeeded | "Payment of $X.XX processed successfully for order #Y" |
| `payment_failed` | Payment failed | "Payment failed for order #Y. Please retry or contact support." |

#### **Message Template Examples:**

**Email Template (Not Sent in Demo):**
```
Subject: Order Confirmation - Order #1

Dear Customer,

Your payment of $999.99 has been processed successfully.

Order Details:
  Order ID: 1
  Transaction ID: TXN-a1b2c3d4
  Amount: $999.99
  Status: Confirmed

Thank you for your business!
```

**SMS Template (Not Sent in Demo):**
```
Your order #1 payment of $999.99 is confirmed. 
Transaction: TXN-a1b2c3d4
```

#### **Dependencies:**
- **None** - Notification Service has no dependencies on other services
- Can run independently
- Optional: Email service, SMS gateway, Slack API (not implemented)

#### **Response Codes:**

| Code | Meaning |
|------|---------|
| 200 | Notification received and processed |
| 400 | Invalid request payload |
| 500 | Internal processing error |

#### **Error Handling:**

| Error | Cause | Action |
|-------|-------|--------|
| Invalid payload | Missing required fields | Return 400 with details |
| Processing failure | Internal error | Log and return 500 |
| Email send failed | Email service unavailable | Log failure, continue |

#### **Logging Pattern:**
```
[NOTIFICATION] Processing payment_success for order 1, trace_id=notification-trace-202
[NOTIFICATION] Received event: TransactionID=TXN-a1b2c3d4, Amount=999.99
[NOTIFICATION] Created NOTIF-12345, status=sent
[NOTIFICATION] Event processed in 12ms
```

#### **In-Memory Storage (Demo Only):**
```go
// Stores notifications per order
notificationLog: {
  1: [
    {
      NotificationID: "NOTIF-12345",
      Type: "payment_success",
      Status: "sent",
      SentAt: "2026-03-02T18:35:30Z",
      TraceID: "notification-trace-202"
    }
  ],
  2: [...]
}

// Resets on pod restart
// In production, use persistent storage or message queue
```

#### **Observability:**
- All events logged with trace_id
- Can be queried in logs using: `kubectl logs -l app=notification-service | grep trace_id`
- Can be integrated with ELK Stack or Loki for log aggregation

#### **Troubleshooting:**
- **Notifications not appearing:** Check pod logs
  ```bash
  kubectl logs notification-service-xxx -n retail | grep "NOTIFICATION"
  ```
- **Trace IDs missing:** Verify Payment Service is passing headers
  ```bash
  kubectl logs notification-service-xxx -n retail | grep "trace_id"
  ```
- **Events being dropped:** Check for error logs
  ```bash
  kubectl logs notification-service-xxx -n retail | grep "ERROR"
  ```

---

### **6. Loyalty Service**

**Purpose:** Customer loyalty points calculation and management  
**Language:** Python 3.11 with FastAPI  
**Location:** http://localhost:5004  
**Port:** 5004  
**Docker Image:** `awezkhan6899/retail-loyalty:latest`  
**Source:** `/loyalty-service/app.py`

#### **Port-Forward:**
```bash
kubectl port-forward svc/loyalty-service 5004:5004 -n retail
```

#### **Core Features:**
1. **Points Calculation Engine**
   - Dynamic multiplier-based earning system
   - Formula: `floor(order_amount × random(1.0 - 1.1))`
   - Converts order total to loyalty points
   - Accounts for different order values

2. **Customer Balance Management**
   - Tracks cumulative points per customer
   - Tier-based benefits (future enhancement)
   - No spending limits
   - In-memory storage for demo

3. **Flexible Integration**
   - Independent service (no dependencies)
   - Can be called for any order amount
   - Returns real-time point calculations
   - Supports point redemption (future)

4. **B3 Header Support**
   - FastAPI middleware extracts trace headers
   - Includes trace_id in all responses
   - Enables distributed tracing (optional)

#### **Environment Variables:**
```bash
PORT=5004               # Loyalty Service port
LOYALTY_MIN_MULTIPLIER=1.0    # Minimum earning rate
LOYALTY_MAX_MULTIPLIER=1.1    # Maximum earning rate
```

#### **Endpoints:**

**POST /calculate-points** - Calculate points for transaction
```bash
curl -X POST http://localhost:5004/calculate-points \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: loyalty-trace-303" \
  -d '{
    "customer_id": "CUST-ABC",
    "order_amount": 999.99
  }'
```
**Request Body:**
```json
{
  "customer_id": "CUST-ABC",
  "order_amount": 999.99,
  "order_id": 1  # Optional
}
```
**Response (200 OK):**
```json
{
  "customer_id": "CUST-ABC",
  "order_amount": 999.99,
  "points_earned": 1089,
  "multiplier": 1.089,
  "timestamp": "2026-03-02T18:35:30Z",
  "total_balance": 5000,
  "trace_id": "loyalty-trace-303"
}
```

**GET /customer/{customer_id}/points** - Get customer balance
```bash
curl http://localhost:5004/customer/CUST-ABC/points \
  -H "x-b3-traceid: loyalty-trace-304"
```
**Response (200 OK):**
```json
{
  "customer_id": "CUST-ABC",
  "total_points": 5089,
  "tier": "Silver",
  "last_activity": "2026-03-02T18:35:30Z",
  "transactions": [
    {
      "order_id": 1,
      "amount": 999.99,
      "points_earned": 1089,
      "earned_at": "2026-03-02T18:35:30Z"
    }
  ],
  "trace_id": "loyalty-trace-304"
}
```

**POST /redeem-points** - Redeem points for discount
```bash
curl -X POST http://localhost:5004/redeem-points \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: loyalty-trace-305" \
  -d '{
    "customer_id": "CUST-ABC",
    "points_to_redeem": 500
  }'
```
**Request Body:**
```json
{
  "customer_id": "CUST-ABC",
  "points_to_redeem": 500
}
```
**Response (200 OK):**
```json
{
  "customer_id": "CUST-ABC",
  "points_redeemed": 500,
  "discount_value": 5.00,
  "remaining_points": 4589,
  "redemption_id": "REDEEM-12345",
  "trace_id": "loyalty-trace-305"
}
```

**GET /health** - Health check
```bash
curl http://localhost:5004/health
```
Returns 200 OK

#### **Points Calculation Logic (Python/FastAPI):**

```python
from fastapi import FastAPI, Header
from typing import Optional
import random
import math

app = FastAPI()

# In-memory customer database
customers_db: dict = {
    "CUST-ABC": {"total_points": 1000, "transactions": []},
    "CUST-XYZ": {"total_points": 500, "transactions": []},
}

LOYALTY_MIN_MULTIPLIER = 1.0
LOYALTY_MAX_MULTIPLIER = 1.1

@app.post("/calculate-points")
async def calculate_points(
    customer_id: str,
    order_amount: float,
    order_id: Optional[int] = None,
    x_b3_traceid: Optional[str] = Header(None)
):
    """
    Calculate loyalty points for an order.
    
    Formula: floor(order_amount × random(1.0 - 1.1))
    
    Example:
      Order Amount: $999.99
      Random Multiplier: 1.089
      Points Earned: floor(999.99 × 1.089) = 1089 points
    """
    
    # Generate random multiplier between 1.0 and 1.1
    multiplier = random.uniform(
        LOYALTY_MIN_MULTIPLIER,
        LOYALTY_MAX_MULTIPLIER
    )
    
    # Calculate points using formula
    raw_points = order_amount * multiplier
    points_earned = math.floor(raw_points)
    
    # Get or create customer
    if customer_id not in customers_db:
        customers_db[customer_id] = {
            "total_points": 0,
            "transactions": []
        }
    
    # Update customer balance
    customer = customers_db[customer_id]
    customer["total_points"] += points_earned
    
    # Record transaction
    transaction = {
        "order_id": order_id,
        "amount": order_amount,
        "points_earned": points_earned,
        "multiplier": round(multiplier, 3),
        "earned_at": datetime.now().isoformat()
    }
    customer["transactions"].append(transaction)
    
    return {
        "customer_id": customer_id,
        "order_amount": order_amount,
        "points_earned": points_earned,
        "multiplier": round(multiplier, 3),
        "total_balance": customer["total_points"],
        "trace_id": x_b3_traceid
    }

@app.get("/customer/{customer_id}/points")
async def get_customer_points(
    customer_id: str,
    x_b3_traceid: Optional[str] = Header(None)
):
    """Retrieve customer loyalty balance and transaction history"""
    
    if customer_id not in customers_db:
        return {"error": "Customer not found"}, 404
    
    customer = customers_db[customer_id]
    
    # Determine tier based on points
    tier = "Bronze"  # 0-1000 points
    if customer["total_points"] >= 5000:
        tier = "Platinum"
    elif customer["total_points"] >= 3000:
        tier = "Gold"
    elif customer["total_points"] >= 1000:
        tier = "Silver"
    
    return {
        "customer_id": customer_id,
        "total_points": customer["total_points"],
        "tier": tier,
        "transactions": customer["transactions"],
        "trace_id": x_b3_traceid
    }

@app.post("/redeem-points")
async def redeem_points(
    customer_id: str,
    points_to_redeem: int,
    x_b3_traceid: Optional[str] = Header(None)
):
    """Redeem points for discount (100 points = $1.00)"""
    
    if customer_id not in customers_db:
        return {"error": "Customer not found"}, 404
    
    customer = customers_db[customer_id]
    
    if customer["total_points"] < points_to_redeem:
        return {
            "error": "Insufficient points",
            "available": customer["total_points"],
            "requested": points_to_redeem
        }, 400
    
    # Redeem points (100 points = $1.00)
    discount_value = points_to_redeem / 100.0
    customer["total_points"] -= points_to_redeem
    
    return {
        "customer_id": customer_id,
        "points_redeemed": points_to_redeem,
        "discount_value": discount_value,
        "remaining_points": customer["total_points"],
        "redemption_id": f"REDEEM-{uuid.uuid4().hex[:12]}",
        "trace_id": x_b3_traceid
    }
```

#### **Points Calculation Examples:**

| Order Amount | Multiplier | Formula | Points Earned |
|--------------|-----------|---------|---------------|
| $100.00 | 1.045 | 100 × 1.045 | 104 |
| $500.00 | 1.087 | 500 × 1.087 | 543 |
| $999.99 | 1.089 | 999.99 × 1.089 | 1089 |
| $1500.00 | 1.001 | 1500 × 1.001 | 1501 |

#### **Loyalty Tiers:**

| Tier | Points Range | Benefits |
|------|-------------|----------|
| Bronze | 0 - 999 | Base earning rate (1.0-1.1x) |
| Silver | 1000 - 2999 | 1.0x earning + 5% bonus |
| Gold | 3000 - 4999 | 1.1x earning + 10% bonus |
| Platinum | 5000+ | 1.2x earning + 15% bonus + VIP support |

#### **Redemption Rules:**
```
100 points = $1.00 discount
Minimum redemption: 100 points
Maximum per redemption: All available points
No expiration date (in demo)
```

#### **Dependencies:**
- **None** - Loyalty Service is completely independent
- No database required (in-memory for demo)
- No dependencies on other services
- Can be scaled independently

#### **Response Codes:**

| Code | Meaning |
|------|---------|
| 200 | Operation successful |
| 400 | Invalid input (insufficient points, etc.) |
| 404 | Customer not found |
| 500 | Internal server error |

#### **Error Scenarios:**

| Scenario | Response |
|----------|----------|
| Customer not found | Create new with 0 points |
| Insufficient points | Return 400 with details |
| Invalid amount | Return 400 with error |

#### **Logging Pattern:**
```python
[LOYALTY] Calculating points for CUST-ABC, order_amount=999.99
[LOYALTY] Generated multiplier: 1.089
[LOYALTY] Points earned: 1089
[LOYALTY] Customer balance updated: 5089, trace_id=loyalty-trace-303
[LOYALTY] Operation completed in 3ms
```

#### **In-Memory Storage (Demo Only):**
```python
# Resets when pod restarts
customers_db = {
    "CUST-ABC": {
        "total_points": 5089,
        "transactions": [
            {
                "order_id": 1,
                "amount": 999.99,
                "points_earned": 1089,
                "earned_at": "2026-03-02T18:35:30Z"
            },
            # ... more transactions
        ]
    },
    "CUST-XYZ": { ... }
}

# For production, use:
# - PostgreSQL with customer_loyalty table
# - Redis for real-time balance cache
# - Event sourcing for transaction audit trail
```

#### **Testing Points Calculation:**
```bash
# Calculate points for $500 order
curl -X POST http://localhost:5004/calculate-points \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"TEST-1","order_amount":500.00}'

# Run multiple times to see variable results (different multipliers)
for i in {1..5}; do
  echo "Attempt $i:"
  curl -X POST http://localhost:5004/calculate-points \
    -H "Content-Type: application/json" \
    -d '{"customer_id":"TEST-MULT","order_amount":1000.00}' | jq .points_earned
done

# Expected output varies (typically 1000-1100 points for $1000 order)
```

#### **Performance Characteristics:**
```
Calculate Points:   ~2-5ms
Get Points:         ~1-2ms
Redeem Points:      ~3-5ms
Memory per Customer: ~1KB (with transaction history)
Max Customers (1GB): ~1,000,000
```

#### **Troubleshooting:**
- **Points always same:** Multiplier randomization may need seed reset
  ```bash
  kubectl logs loyalty-service-xxx -n retail | grep "multiplier"
  ```
- **Customer not found:** Check customer_id format and spelling
  ```bash
  curl http://localhost:5004/customer/CUST-ABC/points
  ```
- **Redemption fails:** Verify sufficient points before redeeming
  ```bash
  curl http://localhost:5004/customer/CUST-ABC/points | jq .total_points
  ```

---

### **7. Admin Frontend (Dashboard)**

**Purpose:** Real-time admin dashboard for system monitoring and management  
**Language:** Go 1.21  
**Location:** http://localhost:8000  
**Port:** 8000  
**Docker Image:** `awezkhan6899/retail-admin-frontend:latest`  
**Source:** `/admin-frontend/main.go`

#### **Port-Forward:**
```bash
kubectl port-forward svc/admin-frontend 8000:8000 -n retail
```

#### **Core Features:**

1. **Real-Time Dashboard**
   - Live statistics on total orders, products, and service health
   - Order management interface with status tracking
   - Inventory management with stock levels
   - Service health monitoring (up/down status)
   - Low stock warnings with reorder functionality

2. **Service Integration**
   - Queries all microservices for data aggregation
   - Displays health status of Order, Inventory, Payment, and Loyalty services
   - Pulls real-time order data from Order Service
   - Fetches inventory catalog from Inventory Service
   - Monitors service availability and response times

3. **Responsive Web UI**
   - Beautiful gradient design with dark theme
   - Interactive tables for orders and inventory
   - Real-time statistics cards
   - Auto-refresh every 30 seconds
   - Manual refresh button for immediate updates

4. **Performance Monitoring**
   - Service response time tracking
   - Query timeout protection (10 seconds)
   - Low stock item alerts (items with <10 units)
   - Recent orders display (last 10 orders)

#### **Environment Variables:**
```bash
PORT=8000                   # Admin Frontend port
ORDER_SERVICE_HOST=order-service    # Order Service hostname
ORDER_SERVICE_PORT=5000     # Order Service port
INVENTORY_SERVICE_HOST=inventory-service  # Inventory hostname
INVENTORY_SERVICE_PORT=5001 # Inventory port
PAYMENT_SERVICE_HOST=payment-service  # Payment hostname
PAYMENT_SERVICE_PORT=5002   # Payment port
LOYALTY_SERVICE_HOST=loyalty-service  # Loyalty hostname
LOYALTY_SERVICE_PORT=5004   # Loyalty port
```

#### **Endpoints:**

**GET /** - Serve Admin Dashboard UI
```bash
# Open in browser or with curl
curl http://localhost:8000
# OR
open http://localhost:8000
```
**Response:** HTML page with interactive admin dashboard

**GET /api/dashboard** - Complete dashboard data (JSON)
```bash
curl http://localhost:8000/api/dashboard
```
**Response:**
```json
{
  "orders": [
    {
      "order_id": 1,
      "customer_id": "CUST-ABC",
      "item_id": "SKU-001",
      "quantity": 2,
      "status": "completed",
      "total_price": 1999.98,
      "created_at": "2026-03-02T18:35:00Z"
    }
  ],
  "inventory": [
    {
      "item_id": "SKU-001",
      "name": "Laptop",
      "description": "High-performance laptop",
      "category": "Electronics",
      "quantity": 50,
      "reserved": 0,
      "price": 999.99
    }
  ],
  "services_health": [
    {
      "service": "Order Service",
      "status": "healthy",
      "port": "5000"
    },
    {
      "service": "Inventory Service",
      "status": "healthy",
      "port": "5001"
    },
    {
      "service": "Payment Service",
      "status": "healthy",
      "port": "5002"
    },
    {
      "service": "Loyalty Service",
      "status": "healthy",
      "port": "5004"
    }
  ],
  "total_orders": 1,
  "total_products": 3,
  "low_stock_items": [],
  "recent_orders": [
    {
      "order_id": 1,
      "customer_id": "CUST-ABC",
      "item_id": "SKU-001",
      "quantity": 2,
      "status": "completed",
      "total_price": 1999.98,
      "created_at": "2026-03-02T18:35:00Z"
    }
  ],
  "timestamp": "2026-03-02T18:40:00Z"
}
```

**GET /api/orders** - Get all orders (JSON)
```bash
curl http://localhost:8000/api/orders
```
**Response:** List of all orders in the system

**GET /api/inventory** - Get all inventory items (JSON)
```bash
curl http://localhost:8000/api/inventory
```
**Response:** Complete inventory catalog with stock levels

**GET /api/services-health** - Get services health status (JSON)
```bash
curl http://localhost:8000/api/services-health
```
**Response:**
```json
[
  {
    "service": "Order Service",
    "status": "healthy",
    "port": "5000"
  },
  {
    "service": "Inventory Service",
    "status": "healthy",
    "port": "5001"
  },
  {
    "service": "Payment Service",
    "status": "healthy",
    "port": "5002"
  },
  {
    "service": "Loyalty Service",
    "status": "healthy",
    "port": "5004"
  }
]
```

**GET /health** - Health check
```bash
curl http://localhost:8000/health
```
Returns 200 OK with `{"status":"healthy"}`

#### **Business Logic (Go):**

```go
// Fetch dashboard data from all services
func handleDashboardAPI(w http.ResponseWriter, r *http.Request) {
  dashboard := DashboardData{
    ServicesHealth: []ServiceHealth{},
    Orders: []Order{},
    Inventory: []InventoryItem{},
    LowStockItems: []InventoryItem{},
    RecentOrders: []Order{},
    Timestamp: time.Now(),
  }

  // Fetch services health (parallel)
  go checkServiceHealth(&dashboard)

  // Fetch orders from Order Service
  ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  defer cancel()
  
  orders := fetchOrders(ctx)
  dashboard.Orders = orders
  dashboard.RecentOrders = orders
  dashboard.TotalOrders = len(orders)

  // Fetch inventory from Inventory Service
  inventory := fetchInventory(ctx)
  dashboard.Inventory = inventory
  dashboard.TotalProducts = len(inventory)

  // Find low stock items (quantity < 10)
  for _, item := range inventory {
    if item.Quantity < 10 {
      dashboard.LowStockItems = append(dashboard.LowStockItems, item)
    }
  }

  w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(dashboard)
}

// Service health check
func checkServiceHealth(dashboard *DashboardData) {
  services := map[string]string{
    "Order Service": fmt.Sprintf("http://%s:%s/health", orderServiceHost, orderServicePort),
    "Inventory Service": fmt.Sprintf("http://%s:%s/health", inventoryServiceHost, inventoryServicePort),
    "Payment Service": fmt.Sprintf("http://%s:%s/health", paymentServiceHost, paymentServicePort),
    "Loyalty Service": fmt.Sprintf("http://%s:%s/health", loyaltyServiceHost, loyaltyServicePort),
  }

  for serviceName, url := range services {
    status := "unhealthy"
    resp, err := http.Get(url)
    if err == nil && resp.StatusCode == 200 {
      status = "healthy"
      resp.Body.Close()
    }
    dashboard.ServicesHealth = append(dashboard.ServicesHealth, ServiceHealth{
      Service: serviceName,
      Status: status,
    })
  }
}
```

#### **Dashboard Features:**

| Feature | Description |
|---------|-------------|
| **Stats Cards** | Shows total orders, products, low stock items, service health |
| **Services Health** | Real-time status of all microservices (color-coded) |
| **Recent Orders** | Last 10 orders with customer, item, total price, status |
| **Inventory Table** | All products with name, price, total stock, reserved, available |
| **Low Stock Alert** | Items with <10 units highlighted with reorder button |
| **Auto-Refresh** | Updates every 30 seconds automatically |
| **Manual Refresh** | Instant data refresh button |

#### **UI Components:**

```html
<!-- Stats Row: 4 metric cards -->
- Total Orders
- Total Products  
- Low Stock Items
- Services Health Status

<!-- Services Table -->
Service Name | Status (Green/Red) | Port

<!-- Recent Orders Table -->
Order ID | Customer | Item | Qty | Total | Status | Created Date

<!-- Inventory Table -->
Item ID | Name | Price | Total Stock | Reserved | Available | Category

<!-- Low Stock Alert Table (if any) -->
Item ID | Name | Price | Stock Level | Reserved | Reorder Button
```

#### **Color Coding:**
- 🟢 **Green** - Healthy / Completed
- 🟡 **Yellow** - Pending / Low Stock
- 🔴 **Red** - Unhealthy / Failed

#### **Dependencies:**
- **Order Service** - For order data (required for dashboard)
- **Inventory Service** - For inventory data (required for dashboard)
- **Payment Service** - For health check only
- **Loyalty Service** - For health check only
- **Kubernetes Network** - Service discovery

#### **Response Codes:**

| Code | Meaning |
|------|---------|
| 200 | Dashboard loaded, data retrieved |
| 503 | One or more services unavailable |
| 504 | Service call timeout |

#### **Performance:**

| Operation | Latency |
|-----------|---------|
| Load Dashboard UI | <100ms |
| Fetch all data | 2-5s (depends on service latency) |
| Service health check | ~1s per service |
| Auto-refresh interval | 30000ms |

#### **Troubleshooting:**

- **Dashboard not loading?** Check browser console for errors
  ```bash
  kubectl logs admin-frontend-xxx -n retail
  ```

- **Services showing as unhealthy?** Verify service endpoints
  ```bash
  curl http://order-service:5000/health
  curl http://inventory-service:5001/health
  ```

- **Orders/Inventory not showing?** Check service connectivity
  ```bash
  kubectl get svc -n retail
  kubectl get endpoints -n retail
  ```

- **Auto-refresh not working?** Check browser network tab
  ```bash
  kubectl logs admin-frontend-xxx -n retail | grep "dashboard"
  ```

---

## 🗄️ Database Access

### **PostgreSQL (Orders)**
**Connection Details (from within cluster):**
```
Host: postgres
Port: 5432
Username: postgres
Password: password
Database: orders_db
```

**Access from local machine (with port-forward):**
```bash
kubectl port-forward svc/postgres 5432:5432 -n retail
psql -h localhost -U postgres -d orders_db
```

**Sample Queries:**
```sql
-- View all orders
SELECT * FROM orders;

-- Find orders by trace_id
SELECT * FROM orders WHERE trace_id = 'abc123xyz';

-- Count orders per item
SELECT item_id, COUNT(*) as order_count 
FROM orders 
GROUP BY item_id;
```

---

### **MongoDB (Inventory)**
**Connection Details (from within cluster):**
```
Host: mongodb
Port: 27017
Database: retail_db
Collection: inventory
```

**Access from local machine (with mongo-shell):**
```bash
kubectl port-forward svc/mongodb 27017:27017 -n retail
mongosh mongodb://localhost:27017/retail_db
```

**Sample Queries:**
```javascript
// View all inventory items
db.inventory.find()

// Find specific item
db.inventory.findOne({ _id: "SKU-001" })

// Update stock
db.inventory.updateOne({ _id: "SKU-001" }, { $set: { quantity: 45 } })
```

---

### **Redis (Sessions)**
**Connection Details (from within cluster):**
```
Host: redis
Port: 6379
```

**Access from local machine (with redis-cli):**
```bash
kubectl port-forward svc/redis 6379:6379 -n retail
redis-cli
```

**Sample Commands:**
```bash
# List all sessions
KEYS session:*

# Get session data
GET session:abc123

# Monitor incoming data
MONITOR
```

---

## 📡 B3 Trace Header Propagation

All services automatically extract and propagate B3 headers for end-to-end tracing:

**Headers involved:**
```
x-request-id              → Unique request identifier
x-b3-traceid              → Root trace ID (stays same across all services)
x-b3-spanid               → Span ID for timing individual operations
x-b3-parentspanid         → Parent span ID for building trace tree
x-b3-sampled              → Whether to sample this trace (0 or 1)
x-b3-flags                → Debug flags
```

**Example trace flow:**
```
Client Request → Frontend (span-1)
                   ↓ (same trace-id, new span-2)
                 Order Service
                   ↓ (same trace-id, new span-3)
                 Inventory Service
                   ↓ (same trace-id, new span-4)
                 PostgreSQL
                 
All logs include: trace-id=abc123, span-id=span-X
```

---

## 🧪 Testing Workflows

### **Test 1: Complete Order Creation**
```bash
# 1. Port-forward services
kubectl port-forward svc/frontend 3000:3000 -n retail &
kubectl port-forward svc/order-service 5000:5000 -n retail &
kubectl port-forward svc/inventory-service 5001:5001 -n retail &

# 2. Create order (with trace ID)
TRACEID="test-order-$(date +%s)"
curl -X POST http://localhost:3000/api/order \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: $TRACEID" \
  -d '{
    "item_id": "SKU-001",
    "quantity": 1,
    "customer_id": "TEST-CUST",
    "total_price": 999.99
  }'

# 3. Check logs
kubectl logs -n retail -l app=order-service | grep "$TRACEID"
kubectl logs -n retail -l app=inventory-service | grep "$TRACEID"
```

### **Test 2: Payment & Notification Flow**
```bash
# Port-forward payment and notification
kubectl port-forward svc/payment-service 5002:5002 -n retail &
kubectl port-forward svc/notification-service 5003:5003 -n retail &

# Process payment
TRACEID="test-payment-$(date +%s)"
curl -X POST http://localhost:5002/process-payment \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: $TRACEID" \
  -d '{
    "order_id": 1,
    "amount": 999.99,
    "customer_id": "TEST-CUST"
  }'

# Check notification logs
kubectl logs -n retail -l app=notification-service | grep "$TRACEID"
```

### **Test 3: Loyalty Points**
```bash
kubectl port-forward svc/loyalty-service 5004:5004 -n retail &

curl -X POST http://localhost:5004/calculate-points \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "TEST-CUST",
    "order_amount": 999.99
  }'
```

### **Test 4: Database Verification**
```bash
# Check PostgreSQL
kubectl exec -it postgres-xxxxx -n retail -- psql -U postgres -d orders_db -c "SELECT * FROM orders;"

# Check MongoDB
kubectl exec -it mongodb-xxxxx -n retail -- mongosh retail_db --eval "db.inventory.find()"

# Check Redis
kubectl exec -it redis-xxxxx -n retail -- redis-cli KEYS "session:*"
```

---

## 🔍 Viewing Logs with Trace Context

**View logs across all services for a specific trace:**
```bash
TRACEID="abc123xyz"
kubectl logs -n retail --all-containers=true -l app=frontend | grep "$TRACEID"
kubectl logs -n retail --all-containers=true -l app=order-service | grep "$TRACEID"
kubectl logs -n retail --all-containers=true -l app=inventory-service | grep "$TRACEID"
kubectl logs -n retail --all-containers=true -l app=payment-service | grep "$TRACEID"
kubectl logs -n retail --all-containers=true -l app=notification-service | grep "$TRACEID"
```

---

## 📝 Service Dependencies Summary

| Service | Type | Depends On | Port |
|---------|------|-----------|------|
| Frontend | Go | Redis, Order Service | 3000 |
| Order Service | Go | PostgreSQL, Inventory Service, Payment Service | 5000 |
| Inventory Service | Python | MongoDB | 5001 |
| Payment Service | Go | Notification Service | 5002 |
| Notification Service | Go | None | 5003 |
| Loyalty Service | Python | None | 5004 |
| Admin Frontend | Go | Order Service, Inventory Service, Payment Service, Loyalty Service | 8000 |

---

## 🚀 Quick Start Commands

```bash
# Port-forward all services
kubectl port-forward svc/frontend 3000:3000 -n retail &
kubectl port-forward svc/order-service 5000:5000 -n retail &
kubectl port-forward svc/inventory-service 5001:5001 -n retail &
kubectl port-forward svc/payment-service 5002:5002 -n retail &
kubectl port-forward svc/notification-service 5003:5003 -n retail &
kubectl port-forward svc/loyalty-service 5004:5004 -n retail &
kubectl port-forward svc/admin-frontend 8000:8000 -n retail &

# Access Frontend UI
open http://localhost:3000

# Access Admin Dashboard
open http://localhost:8000

# Create sample order
curl -X POST http://localhost:3000/api/order \
  -H "Content-Type: application/json" \
  -d '{"item_id":"SKU-001","quantity":1,"customer_id":"DEMO","total_price":999.99}'

# View all pods
kubectl get pods -n retail

# View logs from order service
kubectl logs -n retail -l app=order-service -f

# Kill all port-forwards
pkill -f "kubectl port-forward"
```

---

## 📚 Next Steps

- **Deploy Istio:** Install Istio and enable sidecar injection on `retail-mesh` namespace
- **Configure Jaeger:** Set up distributed tracing to visualize trace flows
- **Traffic Management:** Create VirtualServices for canary deployments, A/B testing
- **Resilience:** Add CircuitBreaker and Retry policies for fault tolerance
