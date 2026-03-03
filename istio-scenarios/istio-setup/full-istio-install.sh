#!/bin/bash

# --- 1. Add Istio Helm Repository ---
echo "Adding Istio Helm repository..."
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update

# --- 2. Create Istio System Namespace ---
echo "Creating istio-system namespace..."
kubectl create namespace istio-system

# --- 3. Install Istio Base (CRDs) ---
echo "Installing Istio Base..."
helm install istio-base istio/base -n istio-system --wait

# --- 4. Install Istiod (Control Plane) ---
echo "Installing Istiod..."
helm install istiod istio/istiod -n istio-system --wait

# --- 5. Install Istio Ingress Gateway ---
echo "Installing Ingress Gateway..."
kubectl create namespace istio-ingress
kubectl label namespace istio-ingress istio-injection=enabled
helm install istio-ingressgateway istio/gateway -n istio-ingress --wait

# --- 6. Install Addons (Prometheus, Grafana, Jaeger, Kiali) ---
# We use the official Istio sample manifests for quick setup
echo "Installing Addons (Prometheus, Grafana, Jaeger, Kiali)..."
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/prometheus.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/grafana.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/jaeger.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/kiali.yaml

echo "======================================================="
echo "Istio Installation Complete!"
echo "Run 'kubectl get pods -n istio-system' to check status."
echo "Run 'istioctl dashboard kiali' to open the UI."
echo "======================================================="
