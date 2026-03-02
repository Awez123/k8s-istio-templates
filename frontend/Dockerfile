# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go files
COPY main.go .

# Download dependencies
RUN go mod init github.com/retail-mesh/frontend && \
    go get github.com/google/uuid && \
    go get github.com/redis/go-redis/v9 && \
    go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o frontend .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/frontend .

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

# Run the service
CMD ["./frontend"]
