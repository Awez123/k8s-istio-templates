package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

type PaymentRequest struct {
	OrderID    int     `json:"order_id"`
	Amount     float64 `json:"amount"`
	CustomerID string  `json:"customer_id"`
}

type PaymentResponse struct {
	Status        string `json:"status"`
	Message       string `json:"message"`
	TransactionID string `json:"transaction_id,omitempty"`
	TraceID       string `json:"trace_id"`
}

type NotificationRequest struct {
	OrderID       int     `json:"order_id"`
	CustomerID    string  `json:"customer_id"`
	TransactionID string  `json:"transaction_id"`
	Message       string  `json:"message"`
	Amount        float64 `json:"amount"`
}

var notificationServiceURL string
var loyaltyServiceURL string
var paymentFailureRate = 0.1 // 10% failure rate for chaos testing

func init() {
	notificationServiceURL = "http://" + getEnv("NOTIFICATION_SERVICE_HOST", "notification-service") + ":" + getEnv("NOTIFICATION_SERVICE_PORT_VAL", "5003")
	loyaltyServiceURL = "http://" + getEnv("LOYALTY_SERVICE_HOST", "loyalty-service") + ":" + getEnv("LOYALTY_SERVICE_PORT_VAL", "5004")

	// Seed random for payment failures
	rand.Seed(time.Now().UnixNano())
}

// propagateHeaders extracts B3 tracing headers from incoming request
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

// getTraceID extracts the trace ID from request headers
func getTraceID(req *http.Request) string {
	if traceID := req.Header.Get("x-b3-traceid"); traceID != "" {
		return traceID
	}
	return "unknown"
}

// processPayment simulates payment processing with a bank gateway
func processPayment(amount float64, traceID string) (bool, string) {
	// Simulate latency (0.5-2 seconds)
	latency := time.Duration(500+rand.Intn(1500)) * time.Millisecond
	time.Sleep(latency)

	// Simulate occasional failures
	if rand.Float64() < paymentFailureRate {
		log.Printf("[Payment Service] Payment processing FAILED for amount=%.2f, TraceID: %s (Simulated failure)", amount, traceID)
		return false, ""
	}

	transactionID := "TXN-" + uuid.New().String()[:8]
	log.Printf("[Payment Service] Payment processed successfully - Amount: %.2f, TransactionID: %s, TraceID: %s", amount, transactionID, traceID)
	return true, transactionID
}

// notifyPaymentSuccess calls Notification Service to send a notification
func notifyPaymentSuccess(traceID string, orderID int, customerID string, transactionID string, amount float64, originalHeaders http.Header) {
	notifReq := NotificationRequest{
		OrderID:       orderID,
		CustomerID:    customerID,
		TransactionID: transactionID,
		Message:       fmt.Sprintf("Payment successful for Order #%d", orderID),
		Amount:        amount,
	}

	body, _ := json.Marshal(notifReq)
	httpReq, err := http.NewRequest(http.MethodPost, notificationServiceURL+"/send-notification", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Payment Service] Failed to create notification request: %v, TraceID: %s", err, traceID)
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
		log.Printf("[Payment Service] Notification Service call failed: %v, TraceID: %s", err, traceID)
		return
	}
	defer resp.Body.Close()

	log.Printf("[Payment Service] Notification sent successfully, TraceID: %s", traceID)
}

// awardLoyaltyPoints calls Loyalty Service to calculate and award points
func awardLoyaltyPoints(traceID string, customerID string, amount float64, originalHeaders http.Header) {
	loyaltyReq := map[string]interface{}{
		"customer_id":  customerID,
		"order_amount": amount,
	}

	body, _ := json.Marshal(loyaltyReq)
	httpReq, err := http.NewRequest(http.MethodPost, loyaltyServiceURL+"/calculate-points", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Payment Service] Failed to create loyalty request: %v, TraceID: %s", err, traceID)
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
		log.Printf("[Payment Service] Loyalty Service call failed: %v, TraceID: %s", err, traceID)
		return
	}
	defer resp.Body.Close()

	log.Printf("[Payment Service] Loyalty points awarded successfully, TraceID: %s", traceID)
}

// processPaymentHandler handles POST /process-payment
func processPaymentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	traceID := getTraceID(r)
	log.Printf("[Payment Service] Received payment request with TraceID: %s", traceID)

	var paymentReq PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&paymentReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Process payment
	success, transactionID := processPayment(paymentReq.Amount, traceID)

	if !success {
		response := PaymentResponse{
			Status:  "failed",
			Message: "Payment declined by bank",
			TraceID: traceID,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Send notification
	go notifyPaymentSuccess(traceID, paymentReq.OrderID, paymentReq.CustomerID, transactionID, paymentReq.Amount, r.Header)

	// Award loyalty points
	go awardLoyaltyPoints(traceID, paymentReq.CustomerID, paymentReq.Amount, r.Header)

	response := PaymentResponse{
		Status:        "success",
		Message:       "Payment processed successfully",
		TransactionID: transactionID,
		TraceID:       traceID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
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
	http.HandleFunc("/process-payment", processPaymentHandler)
	http.HandleFunc("/health", healthHandler)

	port := getEnv("PORT", "5002")
	log.Printf("🚀 Payment Service starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
