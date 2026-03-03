FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy admin-frontend source and go.mod
COPY main.go .
COPY go.mod .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o admin-frontend .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy binary from builder
COPY --from=builder /app/admin-frontend .

# Expose port
EXPOSE 8000

# Set environment variables
ENV PORT=8000 \
    ORDER_SERVICE_HOST=order-service \
    ORDER_SERVICE_PORT=5000 \
    INVENTORY_SERVICE_HOST=inventory-service \
    INVENTORY_SERVICE_PORT=5001 \
    PAYMENT_SERVICE_HOST=payment-service \
    PAYMENT_SERVICE_PORT=5002 \
    LOYALTY_SERVICE_HOST=loyalty-service \
    LOYALTY_SERVICE_PORT=5004

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8000/health || exit 1

# Run application
CMD ["./admin-frontend"]
