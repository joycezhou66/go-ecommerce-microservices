FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /notification-service ./services/notification

FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /notification-service .

EXPOSE 8006

CMD ["./notification-service"]
