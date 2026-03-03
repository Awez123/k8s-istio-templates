# Scenario 3: Circuit Breaking (Service Resilience)

## Goal
Protect the system from a failing **Payment Service** by temporarily removing it from the pool if it starts throwing errors.

## What this does
- If a pod returns **2 consecutive 5xx errors**, it is "ejected" (removed from load balancing) for **30 seconds**.
- Prevents cascading failures when a service pod is unhealthy.

## Steps
1. Apply the circuit breaker:
   ```bash
   kubectl apply -f circuit-breaker.yaml
   ```

## Verification
The Payment Service already has a **10% failure rate** by default. Use `siege` or `curl` to send many requests. Watch the logs—once the limit is hit, Istio will automatically stop sending traffic to the failing pod.
