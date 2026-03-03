# Retail Mesh - Microservices Demo

A complete, 7-microservice retail system designed for Kubernetes and Helm learning. This project demonstrates distributed tracing, inter-service communication, and real database integration (PostgreSQL, MongoDB, Redis).

## 🏗 Architecture & Service Flow

The system simulates a real-world retail flow:
1.  **Frontend (Go):** The customer entry point.
2.  **Order Service (Go):** Manages orders in **PostgreSQL**.
3.  **Inventory Service (Python):** Manages stock in **MongoDB** (Supports atomic stock decrement).
4.  **Payment Service (Go):** Processes payments and triggers downstream events.
5.  **Loyalty Service (Python):** Calculates and awards customer points.
6.  **Notification Service (Go):** Simulates sending alerts.
7.  **Admin Frontend (Go):** A real-time dashboard to monitor service health and inventory.

**Flow:** `Frontend` → `Order Service` → `Inventory (Check/Reserve)` → `Payment` → `Loyalty` & `Notification`.

---

## 🚀 Deployment Options

### 1. Helm (Recommended) -- namespace: retail-mesh
The fastest way to get the entire stack running.
```bash
# Navigate to the chart directory
cd helm-chart

# Install the chart
helm install retail-mesh . -n retail-mesh --create-namespace
```

### 2. Kubernetes Manifests
Standard YAML files organized by service.
```bash
# Create the namespace first
kubectl apply -f k8s-manifests/namespace.yaml

# Apply all manifests
kubectl apply -f k8s-manifests/
```
*Alternatively, use the all-in-one manifest:* `kubectl apply -f gemini-dep.yaml`

### 3. Docker Compose
Perfect for local development without a cluster.
```bash
docker-compose up -d
```

---

## 🔍 Monitoring & Usage

*   **Customer Frontend:** `http://<node-ip>:3000`
*   **Admin Dashboard:** `http://<node-ip>:8000` (View real-time stock and health)
*   **Health Checks:** Every service exposes a `/health` endpoint.

---

## 🛠 Features for Learning
*   **Atomic Transactions:** Inventory Service uses MongoDB `$inc` to ensure stock levels are accurate.
*   **Distributed Tracing:** All services support manual **B3 Header Propagation** (`x-b3-traceid`), making it compatible with Jaeger out-of-the-box.
*   **Istio Ready:** This repository is an excellent playground for practicing **Istio Service Mesh** patterns like Traffic Shifting, Fault Injection, and Mutual TLS (mTLS).

---
*Created for the K8s Community. Happy Deploying!* 🚀
