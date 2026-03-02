package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	orderServiceHost     = getEnv("ORDER_SERVICE_HOST", "order-service")
	orderServicePort     = getEnv("ORDER_SERVICE_PORT", "5000")
	inventoryServiceHost = getEnv("INVENTORY_SERVICE_HOST", "inventory-service")
	inventoryServicePort = getEnv("INVENTORY_SERVICE_PORT", "5001")
	paymentServiceHost   = getEnv("PAYMENT_SERVICE_HOST", "payment-service")
	paymentServicePort   = getEnv("PAYMENT_SERVICE_PORT", "5002")
	loyaltyServiceHost   = getEnv("LOYALTY_SERVICE_HOST", "loyalty-service")
	loyaltyServicePort   = getEnv("LOYALTY_SERVICE_PORT", "5004")
	port                 = getEnv("PORT", "8000")
)

// Data structures for responses
type Order struct {
	OrderID    int       `json:"order_id"`
	CustomerID string    `json:"customer_id"`
	ItemID     string    `json:"item_id"`
	Quantity   int       `json:"quantity"`
	Status     string    `json:"status"`
	TotalPrice float64   `json:"total_price"`
	CreatedAt  time.Time `json:"created_at"`
}

type OrdersResponse struct {
	Orders []Order `json:"orders"`
	Total  int     `json:"total"`
}

type InventoryItem struct {
	ItemID      string  `json:"item_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Quantity    int     `json:"quantity"`
	Reserved    int     `json:"reserved"`
	Price       float64 `json:"price"`
	CreatedAt   string  `json:"created_at"`
}

type ItemsResponse struct {
	Items []InventoryItem `json:"items"`
	Total int             `json:"total"`
}

type ServiceHealth struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Port    string `json:"port"`
}

type DashboardData struct {
	Orders         []Order         `json:"orders"`
	Inventory      []InventoryItem `json:"inventory"`
	ServicesHealth []ServiceHealth `json:"services_health"`
	TotalOrders    int             `json:"total_orders"`
	TotalProducts  int             `json:"total_products"`
	LowStockItems  []InventoryItem `json:"low_stock_items"`
	RecentOrders   []Order         `json:"recent_orders"`
	Timestamp      time.Time       `json:"timestamp"`
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func main() {
	log.Printf("[ADMIN-FRONTEND] Starting Admin Dashboard on port %s", port)
	log.Printf("[ADMIN-FRONTEND] Order Service: %s:%s", orderServiceHost, orderServicePort)
	log.Printf("[ADMIN-FRONTEND] Inventory Service: %s:%s", inventoryServiceHost, inventoryServicePort)
	log.Printf("[ADMIN-FRONTEND] Payment Service: %s:%s", paymentServiceHost, paymentServicePort)
	log.Printf("[ADMIN-FRONTEND] Loyalty Service: %s:%s", loyaltyServiceHost, loyaltyServicePort)

	http.HandleFunc("/", handleDashboard)
	http.HandleFunc("/api/dashboard", handleDashboardAPI)
	http.HandleFunc("/api/orders", handleOrdersAPI)
	http.HandleFunc("/api/inventory", handleInventoryAPI)
	http.HandleFunc("/api/services-health", handleServicesHealth)
	http.HandleFunc("/health", handleHealth)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Serve HTML dashboard
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Retail Admin Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            overflow: hidden;
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        
        .header h1 {
            font-size: 2.5em;
            margin-bottom: 5px;
        }
        
        .header p {
            font-size: 0.9em;
            opacity: 0.9;
        }
        
        .content {
            padding: 30px;
        }
        
        .stats-row {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        
        .stat-card {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            border-radius: 8px;
            text-align: center;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        
        .stat-card .number {
            font-size: 2.5em;
            font-weight: bold;
            margin: 10px 0;
        }
        
        .stat-card .label {
            font-size: 0.9em;
            opacity: 0.9;
        }
        
        .section {
            margin-bottom: 40px;
        }
        
        .section h2 {
            color: #333;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 3px solid #667eea;
        }
        
        table {
            width: 100%;
            border-collapse: collapse;
            background: white;
        }
        
        thead {
            background: #f8f9fa;
            border-bottom: 2px solid #dee2e6;
        }
        
        th {
            padding: 15px;
            text-align: left;
            font-weight: 600;
            color: #333;
        }
        
        td {
            padding: 12px 15px;
            border-bottom: 1px solid #dee2e6;
        }
        
        tbody tr:hover {
            background: #f8f9fa;
        }
        
        .status-badge {
            display: inline-block;
            padding: 5px 10px;
            border-radius: 20px;
            font-size: 0.8em;
            font-weight: 600;
        }
        
        .status-success {
            background: #d4edda;
            color: #155724;
        }
        
        .status-pending {
            background: #fff3cd;
            color: #856404;
        }
        
        .status-failed {
            background: #f8d7da;
            color: #721c24;
        }
        
        .health-good {
            background: #d4edda;
            color: #155724;
        }
        
        .health-bad {
            background: #f8d7da;
            color: #721c24;
        }
        
        .loading {
            text-align: center;
            padding: 40px;
            color: #666;
        }
        
        .spinner {
            border: 4px solid #f3f3f3;
            border-top: 4px solid #667eea;
            border-radius: 50%;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 0 auto 20px;
        }
        
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        
        .error {
            background: #f8d7da;
            color: #721c24;
            padding: 15px;
            border-radius: 5px;
            margin-bottom: 20px;
        }
        
        .low-stock {
            background: #ffe4e1;
        }
        
        .refresh-btn {
            background: #667eea;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 5px;
            cursor: pointer;
            font-size: 0.9em;
            margin-bottom: 20px;
        }
        
        .refresh-btn:hover {
            background: #764ba2;
        }
        
        .footer {
            background: #f8f9fa;
            padding: 20px;
            text-align: center;
            color: #666;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📊 Retail Admin Dashboard</h1>
            <p>Real-time system monitoring and management</p>
        </div>
        
        <div class="content">
            <button class="refresh-btn" onclick="loadDashboard()">🔄 Refresh Data</button>
            <div id="error" class="error" style="display:none;"></div>
            
            <!-- Stats Row -->
            <div class="stats-row">
                <div class="stat-card">
                    <div class="label">Total Orders</div>
                    <div class="number" id="totalOrders">-</div>
                </div>
                <div class="stat-card">
                    <div class="label">Total Products</div>
                    <div class="number" id="totalProducts">-</div>
                </div>
                <div class="stat-card">
                    <div class="label">Low Stock Items</div>
                    <div class="number" id="lowStockCount">-</div>
                </div>
                <div class="stat-card">
                    <div class="label">Services Health</div>
                    <div class="number" id="healthStatus">-</div>
                </div>
            </div>
            
            <!-- Services Health -->
            <div class="section">
                <h2>🔧 Services Health</h2>
                <div id="servicesTable">
                    <div class="loading">
                        <div class="spinner"></div>
                        Loading services...
                    </div>
                </div>
            </div>
            
            <!-- Recent Orders -->
            <div class="section">
                <h2>📋 Recent Orders</h2>
                <div id="ordersTable">
                    <div class="loading">
                        <div class="spinner"></div>
                        Loading orders...
                    </div>
                </div>
            </div>
            
            <!-- Inventory -->
            <div class="section">
                <h2>📦 Inventory Management</h2>
                <div id="inventoryTable">
                    <div class="loading">
                        <div class="spinner"></div>
                        Loading inventory...
                    </div>
                </div>
            </div>
            
            <!-- Low Stock Alert -->
            <div class="section">
                <h2>⚠️ Low Stock Items</h2>
                <div id="lowStockTable">
                    <div class="loading">
                        <div class="spinner"></div>
                        Loading low stock items...
                    </div>
                </div>
            </div>
        </div>
        
        <div class="footer">
            Last updated: <span id="timestamp">-</span>
        </div>
    </div>
    
    <script>
        function formatDate(dateStr) {
            return new Date(dateStr).toLocaleString();
        }
        
        function loadDashboard() {
            fetch('/api/dashboard')
                .then(response => response.json())
                .then(data => {
                    updateStats(data);
                    updateServices(data);
                    updateOrders(data);
                    updateInventory(data);
                    updateLowStock(data);
                    document.getElementById('timestamp').innerText = new Date().toLocaleTimeString();
                })
                .catch(error => {
                    console.error('Error:', error);
                    showError('Failed to load dashboard data: ' + error.message);
                });
        }
        
        function updateStats(data) {
            document.getElementById('totalOrders').innerText = data.total_orders;
            document.getElementById('totalProducts').innerText = data.total_products;
            document.getElementById('lowStockCount').innerText = data.low_stock_items ? data.low_stock_items.length : 0;
            
            let healthy = (data.services_health || []).filter(s => s.status === 'healthy').length;
            document.getElementById('healthStatus').innerText = healthy + '/' + (data.services_health ? data.services_health.length : 0);
        }
        
        function updateServices(data) {
            if (!data.services_health) return;
            
            let html = '<table><thead><tr><th>Service</th><th>Status</th><th>Port</th></tr></thead><tbody>';
            
            data.services_health.forEach(service => {
                let statusClass = service.status === 'healthy' ? 'health-good' : 'health-bad';
                html += '<tr>';
                html += '<td>' + service.service + '</td>';
                html += '<td><span class="status-badge ' + statusClass + '">' + service.status.toUpperCase() + '</span></td>';
                html += '<td>' + service.port + '</td>';
                html += '</tr>';
            });
            
            html += '</tbody></table>';
            document.getElementById('servicesTable').innerHTML = html;
        }
        
        function updateOrders(data) {
            if (!data.recent_orders) {
                document.getElementById('ordersTable').innerHTML = '<p>No orders found</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Order ID</th><th>Customer</th><th>Item</th><th>Quantity</th><th>Total Price</th><th>Status</th><th>Created</th></tr></thead><tbody>';
            
            data.recent_orders.slice(0, 10).forEach(order => {
                let statusClass = 'status-' + order.status;
                html += '<tr>';
                html += '<td>#' + order.order_id + '</td>';
                html += '<td>' + order.customer_id + '</td>';
                html += '<td>' + order.item_id + '</td>';
                html += '<td>' + order.quantity + '</td>';
                html += '<td>$' + order.total_price.toFixed(2) + '</td>';
                html += '<td><span class="status-badge ' + statusClass + '">' + order.status.toUpperCase() + '</span></td>';
                html += '<td>' + formatDate(order.created_at) + '</td>';
                html += '</tr>';
            });
            
            html += '</tbody></table>';
            document.getElementById('ordersTable').innerHTML = html;
        }
        
        function updateInventory(data) {
            if (!data.inventory) {
                document.getElementById('inventoryTable').innerHTML = '<p>No inventory found</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Item ID</th><th>Name</th><th>Price</th><th>Total Stock</th><th>Reserved</th><th>Available</th><th>Category</th></tr></thead><tbody>';
            
            data.inventory.forEach(item => {
                let available = item.quantity - (item.reserved || 0);
                let rowClass = available < 10 ? 'low-stock' : '';
                html += '<tr class="' + rowClass + '">';
                html += '<td>' + item.item_id + '</td>';
                html += '<td>' + item.name + '</td>';
                html += '<td>$' + item.price.toFixed(2) + '</td>';
                html += '<td>' + item.quantity + '</td>';
                html += '<td>' + (item.reserved || 0) + '</td>';
                html += '<td><strong>' + available + '</strong></td>';
                html += '<td>' + item.category + '</td>';
                html += '</tr>';
            });
            
            html += '</tbody></table>';
            document.getElementById('inventoryTable').innerHTML = html;
        }
        
        function updateLowStock(data) {
            if (!data.low_stock_items || data.low_stock_items.length === 0) {
                document.getElementById('lowStockTable').innerHTML = '<p style="color: green; padding: 20px;">✅ All items have healthy stock levels!</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Item ID</th><th>Name</th><th>Price</th><th>Stock Level</th><th>Reserved</th><th>Action</th></tr></thead><tbody>';
            
            data.low_stock_items.forEach(item => {
                html += '<tr class="low-stock">';
                html += '<td>' + item.item_id + '</td>';
                html += '<td>' + item.name + '</td>';
                html += '<td>$' + item.price.toFixed(2) + '</td>';
                html += '<td><strong>' + item.quantity + '</strong></td>';
                html += '<td>' + (item.reserved || 0) + '</td>';
                html += '<td><button onclick="alert(\'Reorder functionality coming soon!\')" style="padding:5px 10px; background:#ff6b6b; color:white; border:none; border-radius:3px; cursor:pointer;">⚠️ Reorder</button></td>';
                html += '</tr>';
            });
            
            html += '</tbody></table>';
            document.getElementById('lowStockTable').innerHTML = html;
        }
        
        function showError(message) {
            let errorDiv = document.getElementById('error');
            errorDiv.innerText = message;
            errorDiv.style.display = 'block';
        }
        
        // Auto-load on page load
        window.onload = function() {
            loadDashboard();
            // Refresh every 30 seconds
            setInterval(loadDashboard, 30000);
        };
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, html)
}

// API endpoint for complete dashboard data
func handleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dashboard := DashboardData{
		ServicesHealth: []ServiceHealth{},
		Orders:         []Order{},
		Inventory:      []InventoryItem{},
		LowStockItems:  []InventoryItem{},
		RecentOrders:   []Order{},
		Timestamp:      time.Now(),
	}

	// Fetch services health
	go checkServiceHealth(&dashboard)

	// Fetch orders
	orders := fetchOrders(ctx)
	dashboard.Orders = orders
	dashboard.RecentOrders = orders
	dashboard.TotalOrders = len(orders)

	// Fetch inventory
	inventory := fetchInventory(ctx)
	dashboard.Inventory = inventory
	dashboard.TotalProducts = len(inventory)

	// Find low stock items
	for _, item := range inventory {
		if item.Quantity < 10 {
			dashboard.LowStockItems = append(dashboard.LowStockItems, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboard)
}

// Fetch orders from Order Service
func fetchOrders(ctx context.Context) []Order {
	url := fmt.Sprintf("http://%s:%s/api/orders?limit=50&offset=0", orderServiceHost, orderServicePort)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[ADMIN-FRONTEND] Error fetching orders: %v", err)
		return []Order{}
	}
	defer resp.Body.Close()

	var ordersResp OrdersResponse
	if err := json.NewDecoder(resp.Body).Decode(&ordersResp); err != nil {
		log.Printf("[ADMIN-FRONTEND] Error decoding orders: %v", err)
		return []Order{}
	}

	return ordersResp.Orders
}

// Fetch inventory from Inventory Service
func fetchInventory(ctx context.Context) []InventoryItem {
	url := fmt.Sprintf("http://%s:%s/items", inventoryServiceHost, inventoryServicePort)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[ADMIN-FRONTEND] Error fetching inventory: %v", err)
		return []InventoryItem{}
	}
	defer resp.Body.Close()

	var itemsResp ItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&itemsResp); err != nil {
		log.Printf("[ADMIN-FRONTEND] Error decoding inventory: %v", err)
		return []InventoryItem{}
	}

	return itemsResp.Items
}

// Check health of all services
func checkServiceHealth(dashboard *DashboardData) {
	services := map[string]string{
		"Order Service":     fmt.Sprintf("http://%s:%s/health", orderServiceHost, orderServicePort),
		"Inventory Service": fmt.Sprintf("http://%s:%s/health", inventoryServiceHost, inventoryServicePort),
		"Payment Service":   fmt.Sprintf("http://%s:%s/health", paymentServiceHost, paymentServicePort),
		"Loyalty Service":   fmt.Sprintf("http://%s:%s/health", loyaltyServiceHost, loyaltyServicePort),
	}

	ports := map[string]string{
		"Order Service":     orderServicePort,
		"Inventory Service": inventoryServicePort,
		"Payment Service":   paymentServicePort,
		"Loyalty Service":   loyaltyServicePort,
	}

	for serviceName, url := range services {
		status := "unhealthy"
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			status = "healthy"
			resp.Body.Close()
		}

		dashboard.ServicesHealth = append(dashboard.ServicesHealth, ServiceHealth{
			Service: serviceName,
			Status:  status,
			Port:    ports[serviceName],
		})
	}
}

// Handle orders API
func handleOrdersAPI(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	orders := fetchOrders(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"orders": orders,
		"total":  len(orders),
	})
}

// Handle inventory API
func handleInventoryAPI(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	inventory := fetchInventory(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": inventory,
		"total": len(inventory),
	})
}

// Handle services health API
func handleServicesHealth(w http.ResponseWriter, r *http.Request) {
	dashboard := &DashboardData{
		ServicesHealth: []ServiceHealth{},
	}

	checkServiceHealth(dashboard)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboard.ServicesHealth)
}

// Health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"healthy"}`)
}
