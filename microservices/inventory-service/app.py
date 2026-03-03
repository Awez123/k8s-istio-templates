import os
import logging
import json
from datetime import datetime, timedelta
from typing import Optional
from uuid import uuid4

from fastapi import FastAPI, Request, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from pymongo import MongoClient
from pymongo.errors import ConnectionFailure

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="Inventory Service")

# Database configuration
MONGO_HOST = os.getenv("MONGO_HOST", "mongodb")
MONGO_PORT = int(os.getenv("MONGO_PORT", 27017))
MONGO_DBNAME = os.getenv("MONGO_DBNAME", "retail_db")

mongo_client = None
db = None

class InventoryItem(BaseModel):
    item_id: str
    name: str
    quantity: int
    price: float

class StockCheckRequest(BaseModel):
    item_id: str
    quantity: int

class StockCheckResponse(BaseModel):
    item_id: str
    available: bool
    in_stock: int
    message: str
    trace_id: str

class ReserveStockRequest(BaseModel):
    item_id: str
    quantity: int

# Middleware for logging trace IDs
@app.middleware("http")
async def log_trace_id(request: Request, call_next):
    trace_id = request.headers.get("x-b3-traceid", "unknown")
    logger.info(f"[Inventory Service] Received request with TraceID: {trace_id}")
    
    response = await call_next(request)
    response.headers["x-b3-traceid"] = trace_id
    return response

# Database initialization
def init_db():
    global mongo_client, db
    
    try:
        mongo_client = MongoClient(
            host=MONGO_HOST,
            port=MONGO_PORT,
            serverSelectionTimeoutMS=5000
        )
        # Test connection
        mongo_client.admin.command('ping')
        db = mongo_client[MONGO_DBNAME]
        logger.info("✓ Connected to MongoDB")
        
        # Create collection and sample data if needed
        if "inventory" not in db.list_collection_names():
            inventory_col = db["inventory"]
            
            # Insert sample items
            sample_items = [
                {
                    "_id": "SKU-001",
                    "name": "Premium Widget",
                    "quantity": 50,
                    "price": 99.99,
                    "created_at": datetime.utcnow()
                },
                {
                    "_id": "SKU-002",
                    "name": "Deluxe Gadget",
                    "quantity": 25,
                    "price": 149.99,
                    "created_at": datetime.utcnow()
                },
                {
                    "_id": "SKU-003",
                    "name": "Standard Device",
                    "quantity": 100,
                    "price": 49.99,
                    "created_at": datetime.utcnow()
                }
            ]
            
            inventory_col.insert_many(sample_items)
            logger.info("✓ Database initialized with sample inventory")
        else:
            logger.info("✓ Inventory collection already exists")
    
    except ConnectionFailure as e:
        logger.fatal(f"Failed to connect to MongoDB: {e}")
        raise

# Initialize DB on startup
@app.on_event("startup")
async def startup_event():
    init_db()

@app.on_event("shutdown")
async def shutdown_event():
    if mongo_client:
        mongo_client.close()
        logger.info("MongoDB connection closed")

@app.get("/health")
async def health_check():
    try:
        # Just check if mongo_client was initialized
        if mongo_client:
            return JSONResponse(status_code=200, content={"status": "OK"})
    except:
        pass
    return JSONResponse(status_code=200, content={"status": "OK"})

@app.post("/check-stock")
async def check_stock(stock_req: StockCheckRequest):
    """Check if item has sufficient stock"""
    # Trace ID is handled by middleware but we can still get it if needed from headers
    # but the logs already show it from middleware.
    
    try:
        inventory_col = db["inventory"]
        item = inventory_col.find_one({"_id": stock_req.item_id})
        
        if not item:
            logger.warning(f"[Inventory Service] Item not found: {stock_req.item_id}")
            return StockCheckResponse(
                item_id=stock_req.item_id,
                available=False,
                in_stock=0,
                message="Item not found",
                trace_id="unknown"
            )
        
        in_stock = item.get("quantity", 0)
        available = in_stock >= stock_req.quantity
        
        logger.info(f"[Inventory Service] Stock check for {stock_req.item_id}: available={available}, in_stock={in_stock}, requested={stock_req.quantity}")
        
        return StockCheckResponse(
            item_id=stock_req.item_id,
            available=available,
            in_stock=in_stock,
            message=f"Item in stock: {in_stock} units" if available else f"Insufficient stock. Available: {in_stock}",
            trace_id="unknown"
        )
    
    except Exception as e:
        logger.error(f"[Inventory Service] Error checking stock: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")

@app.post("/reserve-stock")
async def reserve_stock(reserve_req: ReserveStockRequest):
    """Atomically decrement stock quantity in MongoDB"""
    try:
        inventory_col = db["inventory"]
        
        # Use $inc with a negative value to decrement stock
        # We also check that the current quantity is >= requested quantity
        result = inventory_col.find_one_and_update(
            {"_id": reserve_req.item_id, "quantity": {"$gte": reserve_req.quantity}},
            {"$inc": {"quantity": -reserve_req.quantity}},
            return_document=True
        )
        
        if not result:
            logger.warning(f"[Inventory Service] Failed to reserve stock for {reserve_req.item_id}: Not found or insufficient stock")
            raise HTTPException(status_code=400, detail="Insufficient stock or item not found")
        
        new_quantity = result.get("quantity", 0)
        logger.info(f"[Inventory Service] Stock reserved for {reserve_req.item_id}: new_quantity={new_quantity}")
        
        return {
            "status": "success",
            "item_id": reserve_req.item_id,
            "reserved_quantity": reserve_req.quantity,
            "new_stock": new_quantity,
            "message": "Stock reserved successfully"
        }
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"[Inventory Service] Error reserving stock: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")

@app.get("/items/{item_id}")
async def get_item(item_id: str):
    """Get item details"""
    try:
        inventory_col = db["inventory"]
        item = inventory_col.find_one({"_id": item_id})
        
        if not item:
            logger.warning(f"[Inventory Service] Item not found: {item_id}")
            raise HTTPException(status_code=404, detail="Item not found")
        
        # Return item without MongoDB object ID
        item_data = {
            "item_id": item["_id"],
            "name": item.get("name", ""),
            "quantity": item.get("quantity", 0),
            "price": item.get("price", 0.0),
            "trace_id": "unknown"
        }
        
        logger.info(f"[Inventory Service] Retrieved item {item_id}")
        return item_data
    
    except Exception as e:
        logger.error(f"[Inventory Service] Error retrieving item: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")

@app.get("/items")
async def list_items():
    """List all items"""
    try:
        inventory_col = db["inventory"]
        items = list(inventory_col.find())
        
        items_list = []
        for item in items:
            items_list.append({
                "item_id": item["_id"],
                "name": item.get("name", ""),
                "quantity": item.get("quantity", 0),
                "price": item.get("price", 0.0)
            })
        
        logger.info(f"[Inventory Service] Listed {len(items_list)} items")
        return {"items": items_list, "trace_id": "unknown"}
    
    except Exception as e:
        logger.error(f"[Inventory Service] Error listing items: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")

if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", 5001))
    logger.info(f"🚀 Inventory Service starting on port {port}...")
    uvicorn.run(app, host="0.0.0.0", port=port)
