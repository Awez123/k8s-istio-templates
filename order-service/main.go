package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type Order struct {
	OrderID    int       `json:"order_id"`
	ItemID     string    `json:"item_id"`
	Quantity   int       `json:"quantity"`
	CustomerID string    `json:"customer_id"`
	TotalPrice float64   `json:"total_price"`
	TraceID    string    `json:"trace_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

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

type InventoryCheckRequest struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

type InventoryCheckResponse struct {
	ItemID    string `json:"item_id"`
	Available bool   `json:"available"`
	InStock   int    `json:"in_stock"`
	Message   string `json:"message"`
	TraceID   string `json:"trace_id"`
}

var db *sql.DB
var inventoryServiceURL string
var paymentServiceURL string

func init() {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "postgres"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "retail_user"),
		getEnv("DB_PASSWORD", "retail_password"),
		getEnv("DB_NAME", "retail_db"),
	)

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Test connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	log.Println("✓ Connected to PostgreSQL")

	// Set Service URLs
	inventoryServiceURL = "http://" + getEnv("INVENTORY_SERVICE_HOST", "inventory-service") + ":" + getEnv("INVENTORY_SERVICE_PORT_VAL", "5001")
	paymentServiceURL = "http://" + getEnv("PAYMENT_SERVICE_HOST", "payment-service") + ":" + getEnv("PAYMENT_SERVICE_PORT_VAL", "5002")

	// Create tables
	createTables()
}

func createTables() {
	schema := `
	CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		item_id VARCHAR(255) NOT NULL,
		quantity INT NOT NULL,
		customer_id VARCHAR(255) NOT NULL,
		total_price DECIMAL(10, 2) NOT NULL,
		trace_id VARCHAR(255),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
	log.Println("✓ Database schema initialized")
}

// propagateHeaders extracts B3 tracing headers from incoming request
// and returns them as a map for use in outgoing requests
func propagateHeaders(req *http.Request) http.Header {
	headers := http.Header{}

	// List of B3 headers to propagate
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

// getTraceID extracts the trace ID from request headers
func getTraceID(req *http.Request) string {
	if traceID := req.Header.Get("x-b3-traceid"); traceID != "" {
		return traceID
	}
	return "unknown"
}

// checkInventory calls the Inventory Service to verify stock availability
func checkInventory(traceID string, itemID string, quantity int, originalHeaders http.Header) bool {
	inventoryReq := InventoryCheckRequest{
		ItemID:   itemID,
		Quantity: quantity,
	}

	body, _ := json.Marshal(inventoryReq)
	httpReq, err := http.NewRequest(http.MethodPost, inventoryServiceURL+"/check-stock", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Order Service] Failed to create inventory request: %v", err)
		return false
	}

	// Add headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-b3-traceid", traceID)

	// Propagate other B3 headers if present
	b3Headers := []string{
		"x-request-id",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
	}

	for _, header := range b3Headers {
		if value := originalHeaders.Get(header); value != "" {
			httpReq.Header.Set(header, value)
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[Order Service] Inventory Service call failed: %v, TraceID: %s", err, traceID)
		return false
	}
	defer resp.Body.Close()

	var inventoryResp InventoryCheckResponse
	json.NewDecoder(resp.Body).Decode(&inventoryResp)

	log.Printf("[Order Service] Inventory check result: item=%s, available=%v, in_stock=%d, TraceID: %s", itemID, inventoryResp.Available, inventoryResp.InStock, traceID)

	return inventoryResp.Available
}

// placeOrderHandler handles POST /place-order
func placeOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	traceID := getTraceID(r)
	log.Printf("[Order Service] Received request with TraceID: %s", traceID)

	var orderReq OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check inventory with trace ID propagation
	if !checkInventory(traceID, orderReq.ItemID, orderReq.Quantity, r.Header) {
		log.Printf("[Order Service] Inventory check failed for item %s, TraceID: %s", orderReq.ItemID, traceID)
		response := OrderResponse{
			Status:  "error",
			Message: "Item not in stock",
			TraceID: traceID,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Insert order into database
	var orderID int
	err := db.QueryRow(
		"INSERT INTO orders (item_id, quantity, customer_id, total_price, trace_id) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		orderReq.ItemID,
		orderReq.Quantity,
		orderReq.CustomerID,
		orderReq.TotalPrice,
		traceID,
	).Scan(&orderID)

	if err != nil {
		log.Printf("[Order Service] Database error: %v", err)
		response := OrderResponse{
			Status:  "error",
			Message: "Failed to create order",
			TraceID: traceID,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("[Order Service] Order created successfully - OrderID: %d, TraceID: %s", orderID, traceID)

	// Reserve inventory (decrement stock)
	go reserveInventory(traceID, orderReq.ItemID, orderReq.Quantity, r.Header)

	// Trigger payment asynchronously
	go triggerPayment(traceID, orderID, orderReq.CustomerID, orderReq.TotalPrice, r.Header)

	response := OrderResponse{
		Status:  "success",
		Message: "Order placed successfully",
		OrderID: orderID,
		TraceID: traceID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// triggerPayment calls the Payment Service to process the order payment
func triggerPayment(traceID string, orderID int, customerID string, amount float64, originalHeaders http.Header) {
	paymentReq := map[string]interface{}{
		"order_id":    orderID,
		"customer_id":  customerID,
		"amount":       amount,
	}

	body, _ := json.Marshal(paymentReq)
	httpReq, err := http.NewRequest(http.MethodPost, paymentServiceURL+"/process-payment", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Order Service] Failed to create payment request: %v, TraceID: %s", err, traceID)
		return
	}

	// Add headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-b3-traceid", traceID)

	// Propagate other B3 headers
	b3Headers := []string{
		"x-request-id",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
	}

	for _, header := range b3Headers {
		if value := originalHeaders.Get(header); value != "" {
			httpReq.Header.Set(header, value)
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[Order Service] Payment Service call failed: %v, TraceID: %s", err, traceID)
		return
	}
	defer resp.Body.Close()

	log.Printf("[Order Service] Payment triggered successfully for OrderID: %d, TraceID: %s", orderID, traceID)
}

// reserveInventory calls the Inventory Service to decrement stock quantity
func reserveInventory(traceID string, itemID string, quantity int, originalHeaders http.Header) {
	reserveReq := map[string]interface{}{
		"item_id":  itemID,
		"quantity": quantity,
	}

	body, _ := json.Marshal(reserveReq)
	httpReq, err := http.NewRequest(http.MethodPost, inventoryServiceURL+"/reserve-stock", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Order Service] Failed to create reserve request: %v, TraceID: %s", err, traceID)
		return
	}

	// Add headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-b3-traceid", traceID)

	// Propagate other B3 headers
	b3Headers := []string{
		"x-request-id",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
	}

	for _, header := range b3Headers {
		if value := originalHeaders.Get(header); value != "" {
			httpReq.Header.Set(header, value)
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[Order Service] Reserve stock call failed: %v, TraceID: %s", err, traceID)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Order Service] Stock reservation FAILED with status %d, TraceID: %s", resp.StatusCode, traceID)
		return
	}

	log.Printf("[Order Service] Stock reserved successfully for item %s, TraceID: %s", itemID, traceID)
}

// healthHandler for liveness/readiness probes
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// listOrdersHandler handles GET /api/orders - returns all orders
func listOrdersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query("SELECT id, item_id, quantity, customer_id, total_price, trace_id, created_at FROM orders ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		log.Printf("[Order Service] Query failed: %v", err)
		http.Error(w, "Failed to fetch orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.OrderID, &order.ItemID, &order.Quantity, &order.CustomerID, &order.TotalPrice, &order.TraceID, &order.CreatedAt); err != nil {
			log.Printf("[Order Service] Scan failed: %v", err)
			continue
		}
		order.Status = "completed"
		orders = append(orders, order)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"orders": orders,
		"total":  len(orders),
	})
}

// getOrderHandler handles GET /api/order/{id} - returns specific order
func getOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orderID := r.URL.Query().Get("id")
	if orderID == "" {
		http.Error(w, "Order ID required", http.StatusBadRequest)
		return
	}

	var order Order
	err := db.QueryRow("SELECT id, item_id, quantity, customer_id, total_price, trace_id, created_at FROM orders WHERE id = $1", orderID).
		Scan(&order.OrderID, &order.ItemID, &order.Quantity, &order.CustomerID, &order.TotalPrice, &order.TraceID, &order.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("[Order Service] Query failed: %v", err)
		http.Error(w, "Failed to fetch order", http.StatusInternalServerError)
		return
	}

	order.Status = "completed"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(order)
}

// getEnv retrieves environment variable or returns default
func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

func main() {
	http.HandleFunc("/place-order", placeOrderHandler)
	http.HandleFunc("/api/orders", listOrdersHandler)
	http.HandleFunc("/api/order", getOrderHandler)
	http.HandleFunc("/health", healthHandler)

	port := getEnv("PORT", "5000")
	log.Printf("🚀 Order Service starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
