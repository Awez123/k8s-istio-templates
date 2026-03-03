# Scenario 1: Canary Deployment (Traffic Shifting)

## Goal
Safely test a new version (v2) of the **Frontend** by sending only **10%** of users to it, while keeping **90%** on the stable v1.

## What this does
- **DestinationRule:** Defines two "subsets" (v1 and v2) of the Frontend service based on their deployment labels.
- **VirtualService:** Splitting traffic weights (90/10) between the two subsets.

## Steps
1. Ensure you have two deployments of the frontend (one labeled `version: v1` and another `version: v2`).
2. Apply the canary rule:
   ```bash
   kubectl apply -f canary-frontend.yaml
   ```

## Verification
Refresh the page multiple times. You should see the new version appearing for about 1 out of every 10 requests.
