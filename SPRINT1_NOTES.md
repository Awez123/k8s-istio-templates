# Retail Mesh - Development Notes

## Order Service (Sprint 1)

### Key Implementation Details

#### B3 Header Propagation
```go
func propagateHeaders(req *http.Request) http.Header {
    headers := http.Header{}
    b3Headers := []string{
        "x-request-id",
        "x-b3-traceid",
        "x-b3-spanid",
        "x-b3-parentspanid",
        "x-b3-sampled",
        "x-b3-flags",
    }
    for _, header := range b3Headers {
        if value := req.Header.Get(header); value != "" {
            headers.Set(header, value)
        }
    }
    return headers
}
```

This function extracts all B3 tracing headers from an incoming HTTP request and returns them as a Header map. 
When making outgoing HTTP calls to other services, these headers MUST be attached to preserve the trace context.

#### Trace ID Extraction
- Every request is logged with its `x-b3-traceid`
- Falls back to "unknown" if header is not present
- Critical for correlation across services

#### PostgreSQL Integration
- Connection pooling handled by `database/sql`
- Auto-creates `orders` table on startup
- Each order stores the trace ID for traceability

### Environment Variables
```
PORT=5000                    # Service port
DB_HOST=postgres            # PostgreSQL hostname
DB_PORT=5432               # PostgreSQL port
DB_USER=retail_user        # DB username
DB_PASSWORD=retail_password # DB password
DB_NAME=retail_db          # Database name
```

### Testing Sprint 1

**Create Order:**
```bash
curl -X POST http://localhost:5000/place-order \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: trace-12345" \
  -d '{
    "item_id": "SKU-001",
    "quantity": 1,
    "customer_id": "CUST-001",
    "total_price": 99.99
  }'
```

**Expected Response:**
```json
{
  "status": "success",
  "message": "Order placed successfully",
  "order_id": 1,
  "trace_id": "trace-12345"
}
```

**Verify in Postgres:**
```bash
docker exec -it retail-postgres psql -U retail_user -d retail_db
SELECT id, item_id, customer_id, trace_id FROM orders;
```

### Logs to Monitor

When running `docker-compose up`, you should see:
```
retail-order | ✓ Connected to PostgreSQL
retail-order | ✓ Database schema initialized
retail-order | 🚀 Order Service starting on port 5000...
order-service  | [Order Service] Received request with TraceID: trace-12345
order-service  | [Order Service] Order created successfully - OrderID: 1, TraceID: trace-12345
```

## Notes for Future Sprints

### Sprint 2 (Frontend & Redis)
- Frontend should call Order Service with generated trace ID
- Redis stores session data
- Must propagate trace ID in calls to Order Service

### Sprint 3 (Inventory & MongoDB)
- Order Service should call Inventory Service after placing order
- Must pass B3 headers in HTTP call
- Inventory validates stock and returns availability

### Sprint 4 (Payment, Loyalty, Notification)
- Order Service orchestrates calls to Payment, Loyalty services
- Payment Service calls Notification Service on success
- All services log with received trace ID
