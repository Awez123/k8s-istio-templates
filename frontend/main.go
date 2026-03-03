package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type OrderRequest struct {
	ItemID     string  `json:"item_id"`
	Quantity   int     `json:"quantity"`
	CustomerID string  `json:"customer_id"`
	TotalPrice float64 `json:"total_price"`
}

type OrderResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	OrderID int    `json:"order_id,omitempty"`
	TraceID string `json:"trace_id"`
}

type SessionData struct {
	SessionID  string    `json:"session_id"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
}

type InventoryItem struct {
	ItemID      string  `json:"item_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Quantity    int     `json:"quantity"`
	Reserved    int     `json:"reserved"`
	Price       float64 `json:"price"`
}

type Order struct {
	OrderID    int       `json:"order_id"`
	CustomerID string    `json:"customer_id"`
	ItemID     string    `json:"item_id"`
	Quantity   int       `json:"quantity"`
	UnitPrice  float64   `json:"unit_price"`
	Tax        float64   `json:"tax"`
	TotalPrice float64   `json:"total_price"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	TraceID    string    `json:"trace_id"`
}

type LoyaltyData struct {
	CustomerID   string    `json:"customer_id"`
	TotalPoints  int       `json:"total_points"`
	Tier         string    `json:"tier"`
	LastActivity time.Time `json:"last_activity"`
}

type ServiceHealth struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Port    string `json:"port"`
}

var redisClient *redis.Client
var orderServiceURL string
var inventoryServiceURL string
var paymentServiceURL string
var loyaltyServiceURL string

func init() {
	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr: getEnv("REDIS_HOST", "redis") + ":" + getEnv("REDIS_PORT", "6379"),
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("✓ Connected to Redis")

	// Set Service URLs
	orderServiceURL = "http://" + getEnv("ORDER_SERVICE_HOST", "order-service") + ":" + getEnv("ORDER_SERVICE_PORT_VAL", "5000")
	inventoryServiceURL = "http://" + getEnv("INVENTORY_SERVICE_HOST", "inventory-service") + ":" + getEnv("INVENTORY_SERVICE_PORT_VAL", "5001")
	paymentServiceURL = "http://" + getEnv("PAYMENT_SERVICE_HOST", "payment-service") + ":" + getEnv("PAYMENT_SERVICE_PORT_VAL", "5002")
	loyaltyServiceURL = "http://" + getEnv("LOYALTY_SERVICE_HOST", "loyalty-service") + ":" + getEnv("LOYALTY_SERVICE_PORT_VAL", "5004")

	log.Printf("[FRONTEND] Order Service: %s", orderServiceURL)
	log.Printf("[FRONTEND] Inventory Service: %s", inventoryServiceURL)
	log.Printf("[FRONTEND] Payment Service: %s", paymentServiceURL)
	log.Printf("[FRONTEND] Loyalty Service: %s", loyaltyServiceURL)
}

// propagateHeaders extracts B3 tracing headers from incoming request
// and returns them as a map for use in outgoing requests
func propagateHeaders(req *http.Request) http.Header {
	headers := http.Header{}

	b3Headers := []string{
		"x-request-id",
		"x-b3-traceid",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
	}

	for _, header := range b3Headers {
		if value := req.Header.Get(header); value != "" {
			headers.Set(header, value)
		}
	}

	return headers
}

// getOrCreateTraceID generates a trace ID if not present in request
func getOrCreateTraceID(req *http.Request) string {
	if traceID := req.Header.Get("x-b3-traceid"); traceID != "" {
		return traceID
	}
	// Generate new trace ID
	return uuid.New().String()
}

// createOrGetSession manages Redis session
func createOrGetSession(ctx context.Context, sessionID string) (*SessionData, error) {
	// Try to get existing session
	sessionJSON, err := redisClient.Get(ctx, "session:"+sessionID).Result()
	if err == nil {
		var session SessionData
		if err := json.Unmarshal([]byte(sessionJSON), &session); err == nil {
			// Update last active
			session.LastActive = time.Now()
			sessionBytes, _ := json.Marshal(session)
			redisClient.Set(ctx, "session:"+sessionID, sessionBytes, 24*time.Hour)
			return &session, nil
		}
	}

	// Create new session
	session := SessionData{
		SessionID:  sessionID,
		UserID:     "user-" + uuid.New().String()[:8],
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	sessionBytes, _ := json.Marshal(session)
	err = redisClient.Set(ctx, "session:"+sessionID, sessionBytes, 24*time.Hour).Err()
	if err != nil {
		log.Printf("[Frontend] Failed to create session: %v", err)
		return nil, err
	}

	log.Printf("[Frontend] Session created: %s (UserID: %s)", sessionID, session.UserID)
	return &session, nil
}

// homeHandler serves the modern dashboard UI
func homeHandler(w http.ResponseWriter, r *http.Request) {
	traceID := getOrCreateTraceID(r)
	log.Printf("[FRONTEND] Dashboard request with TraceID: %s", traceID)

	// Get or create session
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := createOrGetSession(ctx, sessionID)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	customerID := session.UserID

	// Use backticks for raw string to avoid escaping issues
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Retail Mesh - Customer Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        .header {
            background: rgba(255,255,255,0.95);
            padding: 25px;
            border-radius: 12px;
            margin-bottom: 30px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.1);
        }
        .header h1 { color: #667eea; margin-bottom: 10px; }
        .session-info {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-top: 15px;
            font-size: 13px;
        }
        .info-box {
            background: #f8f9fa;
            padding: 12px;
            border-radius: 8px;
            border-left: 4px solid #667eea;
        }
        .info-label { font-weight: 600; color: #667eea; }
        .info-value { color: #333; margin-top: 4px; word-break: break-all; font-family: monospace; font-size: 11px; }
        .tabs {
            display: flex;
            gap: 10px;
            margin-bottom: 20px;
            background: rgba(255,255,255,0.95);
            padding: 15px;
            border-radius: 12px;
            flex-wrap: wrap;
        }
        .tab-btn {
            background: #e0e0e0;
            border: none;
            padding: 10px 20px;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 600;
            transition: all 0.3s;
        }
        .tab-btn.active {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
        .tab-btn:hover { transform: translateY(-2px); }
        .tab-content {
            display: none;
            background: rgba(255,255,255,0.95);
            padding: 30px;
            border-radius: 12px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.1);
        }
        .tab-content.active { display: block; }
        .section-title { font-size: 20px; font-weight: 700; color: #333; margin-bottom: 20px; }
        .product-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
            gap: 20px;
        }
        .product-card {
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
            padding: 20px;
            border-radius: 10px;
            border: 2px solid #e0e0e0;
            transition: all 0.3s;
        }
        .product-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 12px 24px rgba(0,0,0,0.15);
            border-color: #667eea;
        }
        .product-title { font-size: 18px; font-weight: 700; color: #333; }
        .product-sku { font-size: 12px; color: #666; margin-top: 8px; }
        .product-stock { font-size: 13px; font-weight: 600; color: #27ae60; margin-top: 10px; }
        .product-price { font-size: 24px; font-weight: 700; color: #667eea; margin: 12px 0; }
        input, select { padding: 10px; border: 2px solid #e0e0e0; border-radius: 8px; margin: 8px 0; width: 100%; }
        input:focus, select:focus { outline: none; border-color: #667eea; box-shadow: 0 0 0 3px rgba(102,126,234,0.1); }
        .btn {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 12px 24px;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 600;
            margin-top: 10px;
            transition: all 0.3s;
        }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 8px 16px rgba(102,126,234,0.3); }
        .btn-secondary { background: #6c757d; }
        .btn-secondary:hover { box-shadow: 0 8px 16px rgba(108,117,125,0.3); }
        .order-card {
            background: #f8f9fa;
            padding: 15px;
            border-left: 4px solid #667eea;
            border-radius: 6px;
            margin-bottom: 15px;
        }
        .order-header { display: flex; justify-content: space-between; margin-bottom: 10px; }
        .order-id { font-weight: 700; color: #333; }
        .order-status {
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: 600;
        }
        .status-pending { background: #fff3cd; color: #856404; }
        .status-completed { background: #d4edda; color: #155724; }
        .status-failed { background: #f8d7da; color: #721c24; }
        .loyalty-card {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            border-radius: 12px;
            margin-bottom: 20px;
        }
        .loyalty-points { font-size: 48px; font-weight: 700; }
        .loyalty-tier { font-size: 18px; margin-top: 10px; opacity: 0.9; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; font-weight: 600; margin-bottom: 5px; color: #333; }
        .response-box {
            background: #f0f0f0;
            padding: 15px;
            border-radius: 8px;
            margin-top: 15px;
            display: none;
            max-height: 300px;
            overflow-y: auto;
            border: 2px solid #e0e0e0;
        }
        .response-box.show { display: block; }
        .response-box.success { background: #d4edda; border-color: #c3e6cb; color: #155724; }
        .response-box.error { background: #f8d7da; border-color: #f5c6cb; color: #721c24; }
        code { background: #f5f5f5; padding: 2px 6px; border-radius: 4px; font-family: monospace; }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 15px;
            margin-bottom: 30px;
        }
        .stat-card {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            border-radius: 10px;
            text-align: center;
        }
        .stat-value { font-size: 32px; font-weight: 700; }
        .stat-label { font-size: 12px; opacity: 0.9; margin-top: 8px; }
        .inventory-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        .inventory-table th {
            background: #667eea;
            color: white;
            padding: 12px;
            text-align: left;
            font-weight: 600;
        }
        .inventory-table td {
            padding: 12px;
            border-bottom: 1px solid #e0e0e0;
        }
        .inventory-table tr:hover { background: #f8f9fa; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Shopping Mall - Customer Dashboard</h1>
            <div class="session-info">
                <div class="info-box">
                    <div class="info-label">Customer ID</div>
                    <div class="info-value">` + customerID + `</div>
                </div>
                <div class="info-box">
                    <div class="info-label">Session ID</div>
                    <div class="info-value">` + sessionID + `</div>
                </div>
                <div class="info-box">
                    <div class="info-label">Trace ID</div>
                    <div class="info-value">` + traceID + `</div>
                </div>
                <div class="info-box">
                    <div class="info-label">Status</div>
                    <div class="info-value">Connected</div>
                </div>
            </div>
        </div>

        <div class="tabs">
            <button class="tab-btn active" onclick="switchTab('shop')">Shop</button>
            <button class="tab-btn" onclick="switchTab('inventory')">Inventory</button>
            <button class="tab-btn" onclick="switchTab('orders')">My Orders</button>
            <button class="tab-btn" onclick="switchTab('loyalty')">Loyalty</button>
        </div>

        <!-- SHOP TAB -->
        <div id="shop" class="tab-content active">
            <div class="section-title">Browse & Purchase Products</div>
            <div class="product-grid">
                <div class="product-card">
                    <div class="product-title">Premium Widget</div>
                    <div class="product-sku">SKU: SKU-001</div>
                    <div class="product-price">$99.99</div>
                    <div class="product-stock">50 in stock</div>
                    <button class="btn" onclick="placeOrder('SKU-001', 'Premium Widget', 1, 99.99)">Buy Now</button>
                </div>
                <div class="product-card">
                    <div class="product-title">Deluxe Gadget</div>
                    <div class="product-sku">SKU: SKU-002</div>
                    <div class="product-price">$149.99</div>
                    <div class="product-stock">25 in stock</div>
                    <button class="btn" onclick="placeOrder('SKU-002', 'Deluxe Gadget', 1, 149.99)">Buy Now</button>
                </div>
                <div class="product-card">
                    <div class="product-title">Pro Tool Set</div>
                    <div class="product-sku">SKU: SKU-003</div>
                    <div class="product-price">$79.99</div>
                    <div class="product-stock">100+ in stock</div>
                    <button class="btn" onclick="placeOrder('SKU-003', 'Pro Tool Set', 1, 79.99)">Buy Now</button>
                </div>
            </div>
            <div id="shop-response" class="response-box"></div>
        </div>

        <!-- INVENTORY TAB -->
        <div id="inventory" class="tab-content">
            <div class="section-title">Product Inventory</div>
            <button class="btn" onclick="fetchInventory()">Refresh Inventory</button>
            <table class="inventory-table" id="inventory-table">
                <thead>
                    <tr>
                        <th>Item ID</th>
                        <th>Product Name</th>
                        <th>Price</th>
                        <th>Available</th>
                        <th>Reserved</th>
                        <th>Category</th>
                    </tr>
                </thead>
                <tbody id="inventory-body">
                    <tr><td colspan="6">Click Refresh to load inventory...</td></tr>
                </tbody>
            </table>
            <div id="inventory-response" class="response-box"></div>
        </div>

        <!-- ORDERS TAB -->
        <div id="orders" class="tab-content">
            <div class="section-title">Order History</div>
            <button class="btn" onclick="fetchMyOrders()">Refresh Orders</button>
            <div id="orders-list" style="margin-top: 20px;"></div>
            <div id="orders-response" class="response-box"></div>
        </div>

        <!-- LOYALTY TAB -->
        <div id="loyalty" class="tab-content">
            <div class="section-title">Loyalty Program</div>
            <div id="loyalty-display"></div>
            <button class="btn" onclick="refreshLoyalty()">Refresh Points</button>
            
            <div style="margin-top: 30px;">
                <h3 style="margin-bottom: 15px;">Redeem Points</h3>
                <div class="form-group">
                    <label>Points to Redeem:</label>
                    <input type="number" id="redeemPoints" placeholder="Enter points" value="100">
                </div>
                <button class="btn" onclick="redeemPoints()">Redeem</button>
            </div>
            <div id="loyalty-response" class="response-box"></div>
        </div>
    </div>

    <script>
        const customerId = '` + customerID + `';

        function switchTab(tabName) {
            document.querySelectorAll('.tab-content').forEach(tab => tab.classList.remove('active'));
            document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
            document.getElementById(tabName).classList.add('active');
            event.target.classList.add('active');
        }

        function showResponse(elementId, message, isSuccess) {
            const element = document.getElementById(elementId);
            if (typeof message === 'string') {
                element.textContent = message;
            } else {
                element.textContent = JSON.stringify(message, null, 2);
            }
            element.classList.add('show');
            element.classList.toggle('success', isSuccess === true);
            element.classList.toggle('error', isSuccess === false);
        }

        async function placeOrder(itemId, itemName, qty, price) {
            try {
                const totalPrice = price * qty;
                const response = await fetch('/api/order', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        item_id: itemId,
                        quantity: qty,
                        customer_id: customerId,
                        total_price: totalPrice
                    })
                });
                const result = await response.json();
                const msg = result.status === 'success' ? 'Order placed! Order ID: ' + (result.order_id || 'Processing...') : 'Order failed';
                showResponse('shop-response', msg, result.status === 'success');
            } catch (error) {
                showResponse('shop-response', 'Error: ' + error.message, false);
            }
        }

        async function fetchInventory() {
            try {
                const response = await fetch('/api/inventory');
                const result = await response.json();
                const tbody = document.getElementById('inventory-body');
                tbody.innerHTML = '';
                
                if (result.items && result.items.length > 0) {
                    result.items.forEach(item => {
                        tbody.innerHTML += '<tr><td>' + item.item_id + '</td><td>' + item.name + '</td><td>$' + item.price.toFixed(2) + '</td><td>' + item.quantity + '</td><td>' + item.reserved + '</td><td>' + (item.category || 'N/A') + '</td></tr>';
                    });
                } else {
                    tbody.innerHTML = '<tr><td colspan="6">No inventory data</td></tr>';
                }
                showResponse('inventory-response', 'Inventory loaded successfully', true);
            } catch (error) {
                showResponse('inventory-response', 'Error: ' + error.message, false);
            }
        }

        async function fetchMyOrders() {
            try {
                const response = await fetch('/api/customer/' + customerId + '/orders');
                const result = await response.json();
                const ordersList = document.getElementById('orders-list');
                ordersList.innerHTML = '';
                
                if (result.orders && result.orders.length > 0) {
                    result.orders.forEach(order => {
                        const statusClass = 'status-' + (order.status || 'pending');
                        const date = new Date(order.created_at).toLocaleDateString();
                        ordersList.innerHTML += '<div class="order-card"><div class="order-header"><div><div class="order-id">Order #' + order.order_id + '</div><div style="font-size: 12px; color: #666; margin-top: 4px;">' + date + '</div></div><div class="order-status ' + statusClass + '">' + (order.status || 'pending').toUpperCase() + '</div></div><div style="font-size: 13px;"><strong>' + order.item_id + '</strong> x ' + order.quantity + ' = <strong>$' + order.total_price.toFixed(2) + '</strong></div></div>';
                    });
                } else {
                    ordersList.innerHTML = '<div class="order-card">No orders yet. Start shopping!</div>';
                }
                showResponse('orders-response', 'Orders loaded', true);
            } catch (error) {
                showResponse('orders-response', 'Error: ' + error.message, false);
            }
        }

        async function refreshLoyalty() {
            try {
                const response = await fetch('/api/loyalty/' + customerId);
                const result = await response.json();
                const display = document.getElementById('loyalty-display');
                
                display.innerHTML = '<div class="loyalty-card"><div class="loyalty-points">' + (result.total_points || 0) + '</div><div class="loyalty-tier">Tier: <strong>' + (result.tier || 'Standard') + '</strong></div><div style="margin-top: 15px; opacity: 0.8; font-size: 12px;">Last Activity: ' + (result.last_activity ? new Date(result.last_activity).toLocaleDateString() : 'None') + '</div></div>';
                showResponse('loyalty-response', 'Loyalty data loaded', true);
            } catch (error) {
                showResponse('loyalty-response', 'Error: ' + error.message, false);
            }
        }

        async function redeemPoints() {
            const points = parseInt(document.getElementById('redeemPoints').value);
            if (!points || points <= 0) {
                showResponse('loyalty-response', 'Please enter valid points', false);
                return;
            }
            try {
                const response = await fetch('/api/loyalty/redeem', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        customer_id: customerId,
                        points_to_redeem: points
                    })
                });
                const result = await response.json();
                showResponse('loyalty-response', 'Redeemed ' + points + ' points! Discount: $' + (result.discount_value || 0), result.status === 'success');
                refreshLoyalty();
            } catch (error) {
                showResponse('loyalty-response', 'Error: ' + error.message, false);
            }
        }

        // Auto-load loyalty on page load
        window.addEventListener('load', function() {
            refreshLoyalty();
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, html)
}

// orderAPIHandler handles POST /api/order - calls Order Service
func orderAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	traceID := getOrCreateTraceID(r)
	log.Printf("[FRONTEND] Order API received request with TraceID: %s", traceID)

	var orderReq OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create request to Order Service
	orderBody, _ := json.Marshal(orderReq)
	orderHTTPReq, err := http.NewRequest(http.MethodPost, orderServiceURL+"/place-order", bytes.NewReader(orderBody))
	if err != nil {
		log.Printf("[FRONTEND] Failed to create request: %v", err)
		http.Error(w, "Failed to create order request", http.StatusInternalServerError)
		return
	}

	// Add headers
	orderHTTPReq.Header.Set("Content-Type", "application/json")
	orderHTTPReq.Header.Set("x-b3-traceid", traceID)

	// Propagate other B3 headers if present
	propagatedHeaders := propagateHeaders(r)
	for key, values := range propagatedHeaders {
		for _, value := range values {
			if key != "x-b3-traceid" { // Already set above
				orderHTTPReq.Header.Set(key, value)
			}
		}
	}

	// Call Order Service
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(orderHTTPReq)
	if err != nil {
		log.Printf("[FRONTEND] Order Service call failed: %v", err)
		response := map[string]interface{}{
			"status":   "error",
			"message":  "Failed to reach Order Service",
			"trace_id": traceID,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, _ := io.ReadAll(resp.Body)
	var orderResp OrderResponse
	json.Unmarshal(body, &orderResp)

	log.Printf("[FRONTEND] Order Service response: OrderID=%v, Status=%s, TraceID=%s", orderResp.OrderID, orderResp.Status, traceID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// healthHandler for liveness/readiness probes
func healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	version := getEnv("APP_VERSION", "v1")
	fmt.Fprintf(w, "OK - %s", version)
}

// inventoryHandler fetches inventory from inventory service
func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inventoryServiceURL+"/items", nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create request"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "Inventory service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var items []InventoryItem
	json.Unmarshal(body, &items)

	respondJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

// customerOrdersHandler fetches orders for a specific customer
func customerOrdersHandler(w http.ResponseWriter, r *http.Request) {
	parts := r.URL.Path
	// Extract customer ID from /api/customer/{id}/orders
	start := len("/api/customer/")
	end := len(parts) - len("/orders")
	if start >= end || end <= start {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "Invalid URL"})
		return
	}

	customerID := parts[start:end]
	log.Printf("[FRONTEND] Fetching orders for customer: %s", customerID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, orderServiceURL+"/api/orders", nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create request"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "Order service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var allOrders map[string]interface{}
	json.Unmarshal(body, &allOrders)

	// Filter orders by customer
	if orders, ok := allOrders["orders"].([]interface{}); ok {
		var customerOrders []map[string]interface{}
		for _, order := range orders {
			if orderMap, ok := order.(map[string]interface{}); ok {
				if custID, ok := orderMap["customer_id"].(string); ok && custID == customerID {
					customerOrders = append(customerOrders, orderMap)
				}
			}
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"orders": customerOrders})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"orders": []interface{}{}})
}

// loyaltyHandler fetches loyalty data for a customer
func loyaltyHandler(w http.ResponseWriter, r *http.Request) {
	customerID := r.URL.Path[len("/api/loyalty/"):]
	log.Printf("[FRONTEND] Fetching loyalty for customer: %s", customerID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loyaltyServiceURL+"/customer/"+customerID+"/points", nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create request"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "Loyalty service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	respondJSON(w, http.StatusOK, result)
}

// loyaltyRedeemHandler redeems loyalty points
func loyaltyRedeemHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bodyBytes, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, loyaltyServiceURL+"/redeem-points", bytes.NewReader(bodyBytes))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create request"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "Loyalty service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	respondJSON(w, resp.StatusCode, result)
}

// paymentHandler processes payment
func paymentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	traceID := getOrCreateTraceID(r)
	var paymentReq map[string]interface{}
	json.NewDecoder(r.Body).Decode(&paymentReq)

	// If order_id not present, use dummy
	if _, ok := paymentReq["order_id"]; !ok {
		paymentReq["order_id"] = 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bodyBytes, _ := json.Marshal(paymentReq)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, paymentServiceURL+"/process-payment", bytes.NewReader(bodyBytes))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create request"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-b3-traceid", traceID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "Payment service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	respondJSON(w, resp.StatusCode, result)
}

// respondJSON helper function
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// getEnv retrieves environment variable or returns default
func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/order", orderAPIHandler)
	http.HandleFunc("/api/inventory", inventoryHandler)
	http.HandleFunc("/api/loyalty/redeem", loyaltyRedeemHandler)
	http.HandleFunc("/api/payment/process", paymentHandler)
	http.HandleFunc("/health", healthHandler)

	// Custom handler for /api/customer/{id}/orders and /api/loyalty/{id}
	http.HandleFunc("/api/customer/", customerOrdersHandler)
	http.HandleFunc("/api/loyalty/", loyaltyHandler)

	port := getEnv("PORT", "3000")
	log.Printf("[FRONTEND] 🚀 Frontend Service starting on port %s...", port)
	log.Printf("[FRONTEND] Order Service: %s", orderServiceURL)
	log.Printf("[FRONTEND] Inventory Service: %s", inventoryServiceURL)
	log.Printf("[FRONTEND] Payment Service: %s", paymentServiceURL)
	log.Printf("[FRONTEND] Loyalty Service: %s", loyaltyServiceURL)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
