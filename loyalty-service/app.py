import os
import logging
import json
from datetime import datetime
from typing import Optional
from random import randint

from fastapi import FastAPI, Request
from pydantic import BaseModel
from pydantic_settings import BaseSettings

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="Loyalty Service")

class LoyaltyCheckRequest(BaseModel):
    customer_id: str
    order_amount: float

class LoyaltyResponse(BaseModel):
    customer_id: str
    points_earned: int
    total_points: int
    message: str
    trace_id: str

# In-memory customer points store
customer_points = {}

# Middleware for logging trace IDs
@app.middleware("http")
async def log_trace_id(request: Request, call_next):
    trace_id = request.headers.get("x-b3-traceid", "unknown")
    logger.info(f"[Loyalty Service] Received request with TraceID: {trace_id}")
    
    response = await call_next(request)
    response.headers["x-b3-traceid"] = trace_id
    return response

@app.get("/health")
async def health_check():
    return {"status": "OK"}

@app.post("/calculate-points")
async def calculate_points(request: Request, loyalty_req: LoyaltyCheckRequest):
    """Calculate loyalty points for a customer"""
    trace_id = request.headers.get("x-b3-traceid", "unknown")
    
    # Calculate points: 1 point per dollar spent (random bonus between 1-10%)
    base_points = int(loyalty_req.order_amount)
    bonus_rate = randint(100, 110) / 100  # 1-10% bonus
    points_earned = int(base_points * bonus_rate)
    
    # Update customer points
    if loyalty_req.customer_id not in customer_points:
        customer_points[loyalty_req.customer_id] = 0
    
    customer_points[loyalty_req.customer_id] += points_earned
    total_points = customer_points[loyalty_req.customer_id]
    
    logger.info(f"[Loyalty Service] Points calculated for {loyalty_req.customer_id}: earned={points_earned}, total={total_points}, TraceID: {trace_id}")
    
    return LoyaltyResponse(
        customer_id=loyalty_req.customer_id,
        points_earned=points_earned,
        total_points=total_points,
        message=f"Earned {points_earned} points! Total: {total_points}",
        trace_id=trace_id
    )

@app.get("/customer/{customer_id}/points")
async def get_customer_points(customer_id: str, request: Request):
    """Get customer's current points"""
    trace_id = request.headers.get("x-b3-traceid", "unknown")
    
    points = customer_points.get(customer_id, 0)
    logger.info(f"[Loyalty Service] Fetched points for {customer_id}: {points}, TraceID: {trace_id}")
    
    return {
        "customer_id": customer_id,
        "total_points": points,
        "trace_id": trace_id
    }

if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", 5004))
    logger.info(f"🚀 Loyalty Service starting on port {port}...")
    uvicorn.run(app, host="0.0.0.0", port=port)
