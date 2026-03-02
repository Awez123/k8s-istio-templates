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

var redisClient *redis.Client
var orderServiceURL string

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

	// Set Order Service URL
	orderServiceURL = "http://" + getEnv("ORDER_SERVICE_HOST", "order-service") + ":" + getEnv("ORDER_SERVICE_PORT", "5000")
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

// homeHandler serves the HTML page with Buy button
func homeHandler(w http.ResponseWriter, r *http.Request) {
	traceID := getOrCreateTraceID(r)
	log.Printf("[Frontend] Received request with TraceID: %s", traceID)

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

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Retail Mesh - Frontend</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 40px;
            background-color: #f5f5f5;
        }
        .container {
            background-color: white;
            padding: 30px;
            border-radius: 8px;
            max-width: 600px;
            margin: 0 auto;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
        }
        .info {
            background-color: #e8f4f8;
            padding: 15px;
            border-radius: 4px;
            margin: 20px 0;
            font-size: 14px;
        }
        .product {
            border: 1px solid #ddd;
            padding: 20px;
            margin: 20px 0;
            border-radius: 4px;
            background-color: #f9f9f9;
        }
        .product h3 {
            margin-top: 0;
        }
        button {
            background-color: #4CAF50;
            color: white;
            padding: 12px 30px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 16px;
            margin-top: 10px;
        }
        button:hover {
            background-color: #45a049;
        }
        .response {
            background-color: #f0f0f0;
            padding: 15px;
            border-radius: 4px;
            margin-top: 20px;
            display: none;
            white-space: pre-wrap;
            word-wrap: break-word;
            font-family: monospace;
        }
        .response.show {
            display: block;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🛍️ Retail Mesh Frontend</h1>
        
        <div class="info">
            <strong>Session ID:</strong> %s<br/>
            <strong>User ID:</strong> %s<br/>
            <strong>Trace ID:</strong> %s<br/>
            <strong>Status:</strong> Connected to backend ✓
        </div>

        <div class="product">
            <h3>Premium Widget - $99.99</h3>
            <p>Item ID: SKU-001</p>
            <p>In Stock: 50 units</p>
            <button onclick="placeOrder('SKU-001', 1, 99.99)">
                🛒 Buy Now
            </button>
        </div>

        <div class="product">
            <h3>Deluxe Gadget - $149.99</h3>
            <p>Item ID: SKU-002</p>
            <p>In Stock: 25 units</p>
            <button onclick="placeOrder('SKU-002', 1, 149.99)">
                🛒 Buy Now
            </button>
        </div>

        <div id="response" class="response"></div>
    </div>

    <script>
        async function placeOrder(itemId, quantity, totalPrice) {
            const responseDiv = document.getElementById('response');
            responseDiv.textContent = 'Processing order...';
            responseDiv.classList.add('show');

            try {
                const orderData = {
                    item_id: itemId,
                    quantity: quantity,
                    customer_id: '%s',
                    total_price: totalPrice
                };

                const response = await fetch('/api/order', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(orderData)
                });

                const result = await response.json();
                responseDiv.textContent = JSON.stringify(result, null, 2);
            } catch (error) {
                responseDiv.textContent = 'Error: ' + error.message;
            }
        }
    </script>
</body>
</html>
    `, sessionID, session.UserID, traceID, session.UserID)

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
	log.Printf("[Frontend] Order API received request with TraceID: %s", traceID)

	var orderReq OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create request to Order Service
	orderBody, _ := json.Marshal(orderReq)
	orderHTTPReq, err := http.NewRequest(http.MethodPost, orderServiceURL+"/place-order", bytes.NewReader(orderBody))
	if err != nil {
		log.Printf("[Frontend] Failed to create request: %v", err)
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
		log.Printf("[Frontend] Order Service call failed: %v", err)
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

	log.Printf("[Frontend] Order Service response: OrderID=%v, Status=%s, TraceID=%s", orderResp.OrderID, orderResp.Status, traceID)

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
	fmt.Fprintf(w, "OK")
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
	http.HandleFunc("/health", healthHandler)

	port := getEnv("PORT", "3000")
	log.Printf("🚀 Frontend Service starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
