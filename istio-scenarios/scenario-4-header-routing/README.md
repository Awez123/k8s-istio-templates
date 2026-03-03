# Scenario 4: Header-Based Routing (Beta Testing)

## Goal
Route users to a "beta" version of the **Order Service** ONLY if their request contains the HTTP header `x-user-type: beta`.

## What this does
- Uses **Layer 7** intelligence to look inside the HTTP request.
- Routes matched users to a beta deployment while regular users remain on the stable version.

## Steps
1. Create a beta deployment of the `order-service` with the label `version: beta`.
2. Apply the routing rules:
   ```bash
   kubectl apply -f header-routing.yaml
   ```

## Verification
Test with two different `curl` commands:
- **Stable:** `curl http://order-service:5000/place-order`
- **Beta:** `curl -H "x-user-type: beta" http://order-service:5000/place-order`
