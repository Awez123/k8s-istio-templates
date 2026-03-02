package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type NotificationRequest struct {
	OrderID       int     `json:"order_id"`
	CustomerID    string  `json:"customer_id"`
	TransactionID string  `json:"transaction_id"`
	Message       string  `json:"message"`
	Amount        float64 `json:"amount"`
}

type NotificationResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
}

// getTraceID extracts the trace ID from request headers
func getTraceID(req *http.Request) string {
	if traceID := req.Header.Get("x-b3-traceid"); traceID != "" {
		return traceID
	}
	return "unknown"
}

// sendNotificationHandler handles POST /send-notification
func sendNotificationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	traceID := getTraceID(r)
	log.Printf("[Notification Service] Received notification request with TraceID: %s", traceID)

	var notifReq NotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&notifReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Simulate sending notification (e.g., email, SMS, push notification)
	log.Printf("[Notification Service] 📧 NOTIFICATION SENT")
	log.Printf("[Notification Service]   OrderID: %d", notifReq.OrderID)
	log.Printf("[Notification Service]   CustomerID: %s", notifReq.CustomerID)
	log.Printf("[Notification Service]   TransactionID: %s", notifReq.TransactionID)
	log.Printf("[Notification Service]   Amount: $%.2f", notifReq.Amount)
	log.Printf("[Notification Service]   Message: %s", notifReq.Message)
	log.Printf("[Notification Service]   TraceID: %s", traceID)

	response := NotificationResponse{
		Status:  "sent",
		Message: "Notification sent successfully",
		TraceID: traceID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// healthHandler for liveness/readiness probes
func healthHandler(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/send-notification", sendNotificationHandler)
	http.HandleFunc("/health", healthHandler)

	port := getEnv("PORT", "5003")
	log.Printf("🚀 Notification Service starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
