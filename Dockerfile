FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the monolithic app
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./main.go

# Final stage
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary
COPY --from=builder /bin/app .

# Copy frontend
COPY frontend/ ./frontend/

EXPOSE 8080

CMD ["./app"]
