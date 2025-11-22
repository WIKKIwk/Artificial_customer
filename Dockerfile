# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
RUN apk add --no-cache build-base sqlite-dev ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# sqlite3 dependency requires CGO
RUN CGO_ENABLED=1 GOOS=linux go build -o bot ./cmd/bot

# Runtime stage
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=builder /app/bot /app/bot
COPY .env.example /app/.env.example
ENV CHAT_DB_PATH=/data/chat.db
VOLUME ["/data"]
CMD ["/app/bot"]
