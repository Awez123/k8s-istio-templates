# Retail Mesh - System Architecture & Trace Flow

## 🏗️ System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         RETAIL MESH                             │
│                    (6 Microservices)                            │
└─────────────────────────────────────────────────────────────────┘

                    ┌──── HTTP/REST ────┐
                    │                   V
           ┌────────────────┐    ┌──────────────┐
           │   Frontend     │    │  Auth        │
           │   (Go, 3000)   │    │  (Sessions)  │
           │                │    │              │
           │ • HTML/UI      │◄──►│  Redis       │
           │ • Sessions     │    │  (6379)      │
           └────────┬────────┘    └──────────────┘
                    │
                    │ x-b3-traceid
                    │ x-b3-spanid (propagated)
                    │
           ┌────────V────────┐     ┌──────────────┐
           │ Order Service   │     │              │
           │ (Go, 5000)      │────►│ PostgreSQL   │
           │                 │     │ (5432)       │
           │ Orchestrator    │     │              │
           │ - Inventory ▼   │     │ • Orders     │
           │ - Payment ▼     │     │ • Trace IDs  │
           │ - Loyalty       │     └──────────────┘
           └────┬────┬────┬──┘
               │    │    │
    ┌──────────┘    │    │
    │   ┌───────────┘    │
    │   │              ┌─┴─────────┐
    │   │              │            │
    V   V              V            V
┌─────────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│ Inventory   │  │ Payment  │  │ Loyalty  │  │Notif'n   │
│ (Python,    │  │ (Go,     │  │(Python,  │  │(Go,      │
│  5001)      │  │ 5002)    │  │ 5004)    │  │5003)     │
│             │  │          │  │          │  │          │
│ ✓ FastAPI   │  │ ✓ Bank   │  │ ✓ Points │  │✓ Async   │
│ ✓ Stock     │  │ ✓ Latency│  │ ✓ Bonus  │  │✓ Event   │
│ ✓ Checks    │  │ ✓ Failure│  │ ✓ Total  │  │✓ Logs    │
└─────┬───────┘  │ ✓ TXN    │  │ ✓ Track  │  │ TraceID  │
      │          │ ✓ TxnID  │  │          │  │          │
      │          └────┬─────┘  └──────────┘  └──────────┘
      │               │
      │               └──► [Notification] ◄──┘
      │                   (async flow)
      │
      V
┌──────────────┐
│  MongoDB     │
│  (27017)     │
│              │
│ • Inventory  │
│ • Items      │
│ • Stock Lvl  │
└──────────────┘
```

---

## 📊 B3 Trace Flow Visualization

### Request #1: Complete Order Flow
```
Trace ID: "final-complete-test"

Frontend Request: x-b3-traceid: final-complete-test
        │
        ├─► [Frontend Service]
        │   └─ Logs: "Received request with TraceID: final-complete-test"
        │   └─ Creates session in Redis
        │   └─ Calls Order Service with B3 headers
        │
        ├─► [Order Service]
        │   └─ Logs: "Received request with TraceID: final-complete-test"
        │   └─ Calls Inventory Service
        │       └─ Propagates: x-b3-traceid: final-complete-test
        │
        │
        ├─► [Inventory Service]
        │   └─ Logs: "Received request with TraceID: final-complete-test"
        │   └─ Stock check returns: available=true
        │   └─ Returns to Order with same trace
        │
        │
        ├─► [Order Service] (cont.)
        │   └─ Saves to PostgreSQL with trace ID
        │   └─ Returns to Frontend with same trace
        │
        │
        └─► [Frontend Service] (response)
            └─ Logs: "Order Service response: TraceID: final-complete-test"
            └─ Returns to client with trace ID in response

TIME: 0.8 seconds
SPAN COUNT: 4 services
DATABASES HIT: 2 (Redis, PostgreSQL, MongoDB)
```

### Request #2: Payment Notification Chain
```
Trace ID: "payment-complete-test-002"

Payment Request: x-b3-traceid: payment-complete-test-002
        │
        ├─► [Payment Service]
        │   └─ Logs: "Received payment request with TraceID: payment-complete-test-002"
        │   └─ Processes payment (1.5-2 seconds simulated)
        │   └─ Calls Notification Service asynchronously
        │       └─ Propagates: x-b3-traceid: payment-complete-test-002
        │
        │
        ├─► [Notification Service] (async)
        │   └─ Logs: "Received notification request with TraceID: payment-complete-test-002"
        │   └─ Prints: "NOTIFICATION SENT"
        │   └─ Includes all order details
        │   └─ Marks transaction as sent
        │
        │
        └─► [Payment Service] (response)
            └─ Logs: "Notification sent successfully, TraceID: payment-complete-test-002"
            └─ Returns to client with same trace

TIME: 2.1 seconds
SPAN COUNT: 3 services
DATABASE HIT: 0 (No DB writes for payment - simulated)
```

---

## 🔍 Current System State (Last Validation)

### Running Containers (9 Total)
```
✅ retail-frontend       (Go, :3000)      - Healthy
✅ retail-order          (Go, :5000)      - Healthy
✅ retail-inventory      (Python, :5001)  - Healthy
✅ retail-payment        (Go, :5002)      - Healthy
✅ retail-notification   (Go, :5003)      - Healthy
✅ retail-loyalty        (Python, :5004)  - Healthy
✅ retail-postgres       (DB, :5432)      - Healthy
✅ retail-mongodb        (DB, :27017)     - Healthy
✅ retail-redis          (Cache, :6379)   - Healthy
```

### Sample Traces Logged
```
Order Created:
  Order ID: 5
  Item: SKU-001
  Quantity: 3
  Total: $299.97
  Trace ID: final-complete-test
  Status: Persisted to PostgreSQL

Payment Processed:
  Transaction ID: TXN-36b2e46f
  Amount: $299.97
  Customer: FINAL-COMPLETE
  Trace ID: payment-complete-test-002
  Status: Success + Notification sent

Loyalty Points:
  Customer: FINAL-COMPLETE
  Points Earned: 299
  Total Points: 299
  Trace ID: points-complete-test
```

---

## Headers Propagation Example

### Request Headers (Going Out)
```
Host: order-service:5000
Content-Type: application/json
x-request-id: req-12345
x-b3-traceid: final-complete-test          ◄── PRIMARY (Never changes)
x-b3-spanid: span-order-001                ◄── Service-specific
x-b3-parentspanid: span-frontend-001       
x-b3-sampled: 1                            ◄── Trace for Jaeger
x-b3-flags: 0
```

### Response Headers (Coming Back)
```
Content-Type: application/json
x-b3-traceid: final-complete-test          ◄── SAME! (Preserved)
```

### Logged Entry (stdout)
```
2026/03/02 17:22:40 [Order Service] Received request with TraceID: final-complete-test
2026/03/02 17:22:40 [Order Service] Inventory check result: item=SKU-001, available=true, in_stock=50, TraceID: final-complete-test
2026/03/02 17:22:40 [Order Service] Order created successfully - OrderID: 5, TraceID: final-complete-test
```

---

## 🎯 Why This Structure Matters for Istio/Jaeger

1. **Single Trace Across Services:** One request creates ONE trace ID that flows through all services
2. **Service Boundaries Clear:** Each span represents one service's work
3. **Parent-Child Relationships:** Frontend -> Order -> Inventory creates clear hierarchy
4. **Async Operations:** Payment -> Notification maintains trace context even for async calls
5. **Database Correlation:** Trace IDs stored in databases enable audit trails
6. **Performance Metrics:** Jaeger will show latency breakdown per service
7. **Failure Correlation:** If Order fails, that failure is tagged with the originating trace

---

## 🚀 Ready for Service Mesh

```
Current Architecture:
  • 6 Services with explicit B3 header handling
  • Databases integrated with trace IDs
  • Async patterns working correctly
  • Error scenarios tested (payment failure simulation)
  • Health checks in place

Next Steps:
  1. kubectl apply -f retail-mesh-deployment.yaml (Istio injected)
  2. Envoy sidecars will intercept HTTP traffic
  3. B3 headers automatically propagated by Istio
  4. Jaeger will receive traces from sidecars
  5. Kiali will visualize the service graph
  6. VirtualServices will enable traffic management
```

---

**Status:** ✅ All services operational with complete trace propagation  
**Ready for:** Istio service mesh deployment  
**Next milestone:** Jaeger integration and visual trace analysis  
