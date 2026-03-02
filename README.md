# Retail Mesh - Microservices Learning Project

A 6-microservice retail system designed to master Istio traffic management, Jaeger distributed tracing, and resilience patterns.

## Project Structure

```
k8s-istio-templates/
├── order-service/          # Orchestrator (Go)
├── frontend/               # Entry point (Go)
├── inventory-service/      # Catalog (Python/FastAPI)
├── payment-service/        # Logic-heavy processor (Go)
├── loyalty-service/        # Points calculator (Python/FastAPI)
├── notification-service/   # Event notifier (Go)
└── docker-compose.yaml
```

## Request Flow (Chain of Command)

```
User Browser (Frontend)
    ↓
Frontend Service (Redis Session)
    ↓
Order Service (PostgreSQL)
    ├→ Inventory Service (MongoDB)
    ├→ Payment Service (Simulate Bank)
    │  └→ Notification Service (Console Log)
    └→ Loyalty Service (Points Calc)
```

## Sprint Progress

### ✅ Sprint 1: Core Order Service & PostgreSQL
- [x] `order-service/main.go` with `/place-order` endpoint
- [x] B3 header propagation helper (`propagateHeaders()`)
- [x] PostgreSQL integration for order persistence
- [x] `order-service/Dockerfile` with multi-stage builds
- [x] Root `docker-compose.yaml` with Postgres & Order service

**Key Features:**
- Logs `x-b3-traceid` for every request
- Auto-creates database schema
- Health check endpoint `/health`
- Extracts and propagates B3 tracing headers

### ⏳ Sprint 2: Frontend & Redis
- [ ] Frontend HTTP server (Go)
- [ ] Redis session management
- [ ] HTML page with "Buy" button

### ⏳ Sprint 3: Inventory Service & MongoDB
- [ ] Inventory service (Python/FastAPI)
- [ ] `/check-stock` endpoint
- [ ] MongoDB integration
- [ ] FastAPI middleware for trace logging

### ⏳ Sprint 4: Payment, Loyalty, Notification
- [ ] Payment service (Go)
- [ ] Loyalty service (Python/FastAPI)
- [ ] Notification service (Go)

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21+ (for local development)
- Python 3.10+ (for local development)

### Run with Docker Compose

```bash
docker-compose up --build
```

### Test Connectivity

```bash
# Create an order
curl -X POST http://localhost:5000/place-order \
  -H "Content-Type: application/json" \
  -H "x-b3-traceid: test-trace-001" \
  -d '{
    "item_id": "item-123",
    "quantity": 2,
    "customer_id": "cust-456",
    "total_price": 49.99
  }'

# Check health
curl http://localhost:5000/health
```

### Inspect Data

```bash
# Access Postgres
docker exec -it retail-postgres psql -U retail_user -d retail_db -c "SELECT * FROM orders;"
```

## B3 Tracing Headers

Every service MUST propagate these headers:
- `x-request-id` - Request correlation ID
- `x-b3-traceid` - Trace ID (same across all services in chain)
- `x-b3-spanid` - Span ID (unique per service)
- `x-b3-parentspanid` - Parent span ID
- `x-b3-sampled` - Sampling decision (0 or 1)
- `x-b3-flags` - Debug flag

**Validation:** If trace ID matches across all 6 services for one request, the system is ready for Jaeger integration.

## Architecture Decisions

- **HTTP/REST** for inter-service communication (not gRPC to keep it simple)
- **PostgreSQL** for transactional data (orders)
- **MongoDB** for catalog/document data (inventory)
- **Redis** for session management
- **Go** for performance-critical services (Order, Payment, Notification)
- **Python/FastAPI** for data-heavy services (Inventory, Loyalty)

## Next Steps

1. **Verify Sprint 1:** Run docker-compose and confirm order creation
2. **Implement Sprint 2:** Add Frontend and Redis
3. **Implement Sprint 3:** Add Inventory and MongoDB
4. **Implement Sprint 4:** Add Payment, Loyalty, and Notification
5. **Deploy to Istio:** Apply the learned patterns to a K8s cluster with Istio
