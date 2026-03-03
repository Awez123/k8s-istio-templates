# Scenario 2: Fault Injection (Chaos Engineering)

## Goal
Test how the **Order Service** behaves when the **Inventory Service** is slow.

## What this does
- Injects a **5-second delay** into **100%** of requests to the Inventory Service.
- Simulates network latency or high database load.

## Steps
1. Apply the fault injection rule:
   ```bash
   kubectl apply -f fault-injection.yaml
   ```

## Verification
Try to place an order via the Frontend or using `curl`. You will notice the request hangs for exactly 5 seconds before completing or timing out.
