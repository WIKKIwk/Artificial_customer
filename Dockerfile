FROM golang:1.22-alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot

FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/bot /app/bot

CMD ["/app/bot"]
