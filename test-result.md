# Istio Scenarios - Test Results

This file documents the execution and verification of Istio service mesh scenarios.

## 📋 System Environment Check
- **Namespace:** `retail-mesh`
- **Istio Injection:** Enabled
- **Addons:** Kiali, Jaeger, Grafana, Prometheus (installed)

---
## 🚀 Scenario 1: Canary Deployment (Traffic Shifting)
**Goal:** 90% traffic to `v1`, 10% to `v2`.

### Steps:
1. Updated `frontend` deployment with `version: v1` label and `APP_VERSION=v1`.
2. Deployed `frontend-v2` with `version: v2` label and `APP_VERSION=v2`.
3. Applied `DestinationRule` and `VirtualService`.
4. Executed 20 requests from `admin-frontend` pod.

### Commands:
```bash
kubectl apply -f istio-scenarios/scenario-1-canary/canary-frontend.yaml
kubectl exec -n retail-mesh -it <admin-pod> -- sh -c "for i in \$(seq 1 20); do wget -qO- http://frontend:3000/health; echo ''; done"
```

### Results:
- **v1 Responses:** 18 (90%)
- **v2 Responses:** 2 (10%)
- **Status:** ✅ SUCCESS

---
## 🚀 Scenario 2: Fault Injection (Chaos Engineering)
**Goal:** Inject 5s delay into `inventory-service` requests.

### Steps:
1. Applied `VirtualService` with `fault.delay`.
2. Measured response time using `time wget`.

### Commands:
```bash
kubectl apply -f istio-scenarios/scenario-2-fault-injection/fault-injection.yaml
kubectl exec -n retail-mesh -it <admin-pod> -- sh -c "time wget -qO- http://inventory-service:5001/health"
```

### Results:
- **Response Time:** 5.02s
- **Expected Delay:** 5s
- **Status:** ✅ SUCCESS

---
## 🚀 Scenario 3: Circuit Breaking (Service Resilience)
**Goal:** Eject `payment-service` if it returns 2 consecutive 5xx errors.

### Steps:
1. Updated `payment-service` to return `503 Service Unavailable` on bank failures (simulating real server errors).
2. Applied `DestinationRule` with `outlierDetection`.
3. Sent 50 requests to trigger the 10% failure rate.

### Commands:
```bash
kubectl apply -f istio-scenarios/scenario-3-circuit-breaking/circuit-breaker.yaml
```

### Results:
- **Observation:** After a few errors, Istio "ejected" the pod, and a consecutive block of `503` errors was returned (fast-fail) instead of trying the pod again.
- **Status:** ✅ SUCCESS

---
## 🚀 Scenario 4: Header-Based Routing (Beta Testing)
**Goal:** Route users to `beta` version of `order-service` if header `x-user-type: beta` is present.

### Steps:
1. Updated `order-service` to return its version in `/health`.
2. Deployed `order-service-beta`.
3. Applied `VirtualService` with header matching rules.

### Commands:
```bash
kubectl apply -f istio-scenarios/scenario-4-header-routing/header-routing.yaml
kubectl exec -n retail-mesh -it <admin-pod> -- wget -qO- http://order-service:5000/health
kubectl exec -n retail-mesh -it <admin-pod> -- wget -qO- --header="x-user-type: beta" http://order-service:5000/health
```

### Results:
- **Normal Request:** `OK - stable`
- **Beta Header Request:** `OK - beta`
- **Status:** ✅ SUCCESS

---
## 🚀 Scenario 5: Zero-Trust Security (mTLS)
**Goal:** Enforce STRICT mTLS for all communication.

### Steps:
1. Applied `PeerAuthentication` with mode `STRICT`.
2. Tested connection from inside mesh (sidecar pod).
3. Tested connection from outside mesh (pod without sidecar).

### Commands:
```bash
kubectl apply -f istio-scenarios/scenario-5-mtls/mtls-strict.yaml
```

### Results:
- **Internal Access:** ✅ Success (Sidecars handle mTLS handshake)
- **External Access:** ❌ Blocked (Error: `Connection reset by peer`)
- **Status:** ✅ SUCCESS

---
**Summary:** All 5 Istio scenarios were successfully implemented, tested, and verified using the `retail-mesh` microservices stack.
