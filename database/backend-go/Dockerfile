# Build Stage
FROM golang:alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download
RUN go mod tidy

# Copy source code
COPY . .

# Build the binary (main.go with v2.3.0 features)
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Final Stage
FROM alpine:latest

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .
COPY --from=builder /app/.env .

# Expose port
EXPOSE 8080

# Run
CMD ["./main"]
