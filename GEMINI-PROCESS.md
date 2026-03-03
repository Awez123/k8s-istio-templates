# Retail Mesh - Bug Fix & Deployment Process

## Identified Issues
1. **Inventory Service (422 Unprocessable Entity):** `POST /check-stock` fails with validation errors.
2. **Loyalty Service (404 Not Found):** Missing routes or mismatched paths when called from the frontend (specifically `/redeem-points` and potentially others).
3. **Tracing/Flow Gaps:** Loyalty points calculation might not be integrated into the order/payment flow as intended.

## Plan
### 1. Research & Fixes
- [ ] **Inventory Service:** 
    - Investigate `app.py` for `check_stock` parameter issues.
    - Verify JSON body alignment.
- [ ] **Loyalty Service:**
    - Add missing `/redeem-points` endpoint.
    - Ensure `/calculate-points` is being called correctly by the system.
    - Verify path consistency between `frontend/main.go` and `loyalty-service/app.py`.
- [ ] **System Integration:**
    - Ensure `loyalty-service` is called after successful payment or order.

### 2. Docker Build & Push
- [ ] Build images for modified services:
    - `awezkhan6899/retail-inventory:latest`
    - `awezkhan6899/retail-loyalty:latest`
    - (Any others modified)
- [ ] Push to Docker Hub.

### 3. K8s Update & Validation
- [ ] Update `gemini-dep.yaml` with any new configuration.
- [ ] Apply changes to the cluster.
- [ ] Validate end-to-end flow using the debug pod.

---
## Progress Log
- [2026-03-03] Initial deployment successful, identified 422 in Inventory and 404 in Loyalty.
