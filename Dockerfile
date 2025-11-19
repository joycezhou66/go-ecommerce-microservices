FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all services
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/user-service ./services/user
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/product-service ./services/product
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/cart-service ./services/cart
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/order-service ./services/order
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/payment-service ./services/payment
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/notification-service ./services/notification
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/gateway ./services/gateway

# Final stage
FROM alpine:3.18

RUN apk --no-cache add ca-certificates supervisor

WORKDIR /app

# Copy binaries
COPY --from=builder /bin/user-service .
COPY --from=builder /bin/product-service .
COPY --from=builder /bin/cart-service .
COPY --from=builder /bin/order-service .
COPY --from=builder /bin/payment-service .
COPY --from=builder /bin/notification-service .
COPY --from=builder /bin/gateway .

# Copy frontend
COPY frontend/ ./frontend/

# Copy supervisor config
COPY supervisord.conf /etc/supervisord.conf

EXPOSE 8080

CMD ["supervisord", "-c", "/etc/supervisord.conf"]
