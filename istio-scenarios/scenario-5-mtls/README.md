# Scenario 5: Zero-Trust Security (mTLS)

## Goal
Enforce **Strict Mutual TLS** (mTLS) for all communication within the `retail-mesh` namespace to ensure only authenticated services can communicate.

## What this does
- Changes the mTLS mode from `PERMISSIVE` to `STRICT`.
- Traffic that is not encrypted and authenticated by Istio will be **rejected**.
- Ensures a **Zero-Trust** environment where no service is trusted by default.

## Steps
1. Apply the strict mTLS policy:
   ```bash
   kubectl apply -f mtls-strict.yaml
   ```

## Verification
Try to access a service from outside the mesh or using a pod without an Istio sidecar. The connection will be refused. Within the mesh, services will continue to communicate securely and transparently.
