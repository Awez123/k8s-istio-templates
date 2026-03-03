# Istio Base Setup Guide

This folder contains the foundation for all Istio practice scenarios, including a full stack installation.

## 0. Full Istio Installation (with Addons)
To install Istio along with **Kiali, Jaeger, Grafana, and Prometheus**, run the provided script:
```bash
# Make script executable (Linux/Mac)
chmod +x full-istio-install.sh

# Run the installation
./full-istio-install.sh
```
*Note: This script uses Helm for the core and official samples for the addons.*

## 1. Enable Istio Injection
Tag the namespace so Istio automatically adds sidecar proxies (Envoy) to every pod.
```bash
kubectl apply -f namespace-setup.yaml
```

## 2. Setup the Ingress Gateway
Create the gateway to allow external traffic into the cluster.
```bash
kubectl apply -f gateway.yaml
```

## 3. Configure Root Routing
Route traffic from the Gateway to the appropriate services (Frontend, Admin, Order).
```bash
kubectl apply -f root-virtualservice.yaml
```

---
## 📊 Accessing Dashboards
Once the addons are installed, you can access the dashboards using the `istioctl` CLI:
- **Kiali (Service Map):** `istioctl dashboard kiali`
- **Jaeger (Tracing):** `istioctl dashboard jaeger`
- **Grafana (Metrics):** `istioctl dashboard grafana`

---
**Verification:**
After applying these, find your Istio Ingress Gateway IP and try to access the `frontend` in your browser.
