# 🧭 The Ultimate Istio Practice Guide

This guide provides a deep dive into the 5 most common Service Mesh patterns using the **Retail Mesh** system.

---

## 🛠 Prerequisites & Initial Setup

Before you start, you must have Istio and the base Retail Mesh deployed.

1.  **Install Istio & Addons:**
    ```bash
    cd istio-scenarios/istio-setup
    chmod +x full-istio-install.sh
    ./full-istio-install.sh
    ```
2.  **Deploy Retail Mesh:**
    ```bash
    # From the project root
    kubectl apply -f gemini-dep.yaml
    ```
3.  **Label Namespace for Injection:**
    ```bash
    kubectl label namespace retail-mesh istio-injection=enabled --overwrite
    # Restart pods to pick up the sidecar
    kubectl rollout restart deployment -n retail-mesh
    ```

---

## 🏗 STEP 1: Deploying Multiple Versions (The "v2" Logic)
For Istio to route traffic between "v1" and "v2", you need pods running both versions. We achieve this by running two deployments that share the same **Service**, but have different **Version Labels**.

**Apply the Multi-Version manifest:**
```bash
# This creates frontend-v2 and order-service-beta
kubectl apply -f istio-scenarios/test-pre-reqs.yaml
```

### 💡 Why do we do this?
Kubernetes Services load-balance across all pods matching a label (e.g., `app: frontend`). Istio takes control by looking at a secondary label (e.g., `version: v1` vs `version: v2`) to decide exactly which pod gets which request.

---

## 🌐 STEP 2: Configure Ingress (The Entry Point)
Tell Istio to allow traffic into the cluster and route it to our services.
```bash
kubectl apply -f istio-scenarios/istio-setup/gateway.yaml
kubectl apply -f istio-scenarios/istio-setup/root-virtualservice.yaml
```

---

## 🚀 Scenario 1: Canary Deployment (Weight-Based Routing)
**The Concept:** You have a new frontend (v2) and you want to test it on only 10% of users.

1.  **Apply:** `kubectl apply -f istio-scenarios/scenario-1-canary/canary-frontend.yaml`
2.  **Generate Traffic:**
    ```bash
    kubectl exec -n retail-mesh -it $(kubectl get pod -l app=admin-frontend -n retail-mesh -o jsonpath='{.items[0].metadata.name}') -- sh -c "while true; do wget -qO- http://frontend:3000/health; echo ''; sleep 0.2; done"
    ```
3.  **Monitor in KIALI:**
    - Run `istioctl dashboard kiali -n istio-system`.
    - Select **Graph** -> Namespace: `retail-mesh`.
    - In the **Display** dropdown, check **Request Distribution**.
    - **Result:** You will see the arrow splitting: `90%` to `frontend-v1` and `10%` to `frontend-v2`.

---

## 🚀 Scenario 2: Fault Injection (Injected Latency)
**The Concept:** Test if the `order-service` crashes if the `inventory-service` takes too long to respond.

1.  **Apply:** `kubectl apply -f istio-scenarios/scenario-2-fault-injection/fault-injection.yaml`
2.  **Test:**
    ```bash
    kubectl exec -n retail-mesh -it $(kubectl get pod -l app=admin-frontend -n retail-mesh -o jsonpath='{.items[0].metadata.name}') -- sh -c "time wget -qO- http://inventory-service:5001/health"
    ```
3.  **Monitor in JAEGER:**
    - Run `istioctl dashboard jaeger -n istio-system`.
    - Search for Service: `order-service`.
    - **Result:** You will see a "long" trace. Clicking it reveals that the sub-call to `inventory-service` spent **5 seconds** waiting (the injected fault).

---

## 🚀 Scenario 3: Circuit Breaking (Self-Healing)
**The Concept:** If a service pod is failing, "eject" it so users don't see errors while the pod recovers or is replaced.

1.  **Apply:** `kubectl apply -f istio-scenarios/scenario-3-circuit-breaking/circuit-breaker.yaml`
2.  **Trigger:**
    ```bash
    # Payment service has a 10% error rate. This storm triggers the breaker.
    kubectl exec -n retail-mesh -it $(kubectl get pod -l app=admin-frontend -n retail-mesh -o jsonpath='{.items[0].metadata.name}') -- sh -c "for i in \$(seq 1 50); do wget -qO- --post-data='{\"amount\": 10}' --header='Content-Type: application/json' http://payment-service:5002/process-payment; done"
    ```
3.  **Monitor in GRAFANA:**
    - Run `istioctl dashboard grafana -n istio-system`.
    - Open **Istio Service Dashboard** -> Service: `payment-service`.
    - **Result:** You will see the **Success Rate** drop, then the **Incoming Requests** to that pod flatline as Istio breaks the circuit to protect the mesh.

---

## 🚀 Scenario 4: Header-Based Routing (Beta Testing)
**The Concept:** Only users with a special header (like a Cookie or User-Type) get to see the "Beta" Order Service.

1.  **Apply:** `kubectl apply -f istio-scenarios/scenario-4-header-routing/header-routing.yaml`
2.  **Test:**
    - **Regular:** `wget -qO- http://order-service:5000/health` -> Returns `OK - stable`
    - **Beta:** `wget -qO- --header="x-user-type: beta" http://order-service:5000/health` -> Returns `OK - beta`
3.  **Monitor in KIALI:**
    - You will see two different paths light up in the graph depending on which `wget` command you run.

---

## 🚀 Scenario 5: Zero-Trust Security (Strict mTLS)
**The Concept:** Encrypt all traffic and reject any pod that doesn't have an Istio sidecar.

1.  **Apply:** `kubectl apply -f istio-scenarios/scenario-5-mtls/mtls-strict.yaml`
2.  **Test:**
    - **Mesh Pod:** `kubectl exec ... -- wget` (Works)
    - **Non-Mesh Pod:** `kubectl run non-mesh --image=alpine -- wget ...` (Fails with `Connection reset`)
3.  **Monitor in KIALI:**
    - Click **Display** -> **Security**.
    - **Result:** Small **Locks 🔒** will appear on all lines in the graph, confirming encrypted mTLS tunnels.

---

## 🧹 Cleanup
To reset everything and start fresh:
```bash
kubectl delete vs,dr,pa --all -n retail-mesh
```
