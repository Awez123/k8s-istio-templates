.PHONY: help build up down logs test clean restart

help:
	@echo "Retail Mesh - Development Commands"
	@echo "===================================="
	@echo "make build      - Build all Docker images"
	@echo "make up         - Start all services"
	@echo "make down       - Stop all services"
	@echo "make restart    - Restart all services"
	@echo "make logs       - Show logs from all services"
	@echo "make test       - Run connectivity tests"
	@echo "make clean      - Clean up all Docker resources"
	@echo "make db-shell   - Open PostgreSQL shell"
	@echo "make db-reset   - Reset database (drop all data)"

build:
	docker-compose build

up:
	docker-compose up -d

down:
	docker-compose down

restart: down up
	@echo "Services restarted"

logs:
	docker-compose logs -f

logs-order:
	docker-compose logs -f order-service

test:
	@echo "Testing Order Service..."
	@curl -s -X POST http://localhost:5000/place-order \
		-H "Content-Type: application/json" \
		-H "x-b3-traceid: test-trace-$(shell date +%s)" \
		-d '{"item_id":"TEST-001","quantity":1,"customer_id":"TEST-CUST","total_price":99.99}' | jq .
	@echo "\n✓ Test completed"

health:
	@echo "Checking service health..."
	@curl -s http://localhost:5000/health && echo "\n✓ Order Service is healthy"

db-shell:
	docker exec -it retail-postgres psql -U retail_user -d retail_db

db-reset:
	docker exec -it retail-postgres psql -U retail_user -d retail_db -c "DROP TABLE IF EXISTS orders; COMMIT;"
	@echo "Database reset complete"

db-show:
	docker exec -it retail-postgres psql -U retail_user -d retail_db -c "SELECT id, item_id, quantity, customer_id, total_price, trace_id, created_at FROM orders ORDER BY id DESC LIMIT 10;"

clean:
	docker-compose down -v
	@echo "Cleaned up all Docker resources"

logs-follow:
	docker-compose logs -f --tail=50
