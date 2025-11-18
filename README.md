# GoShop - E-Commerce Microservices Platform

A scalable e-commerce platform built with Go microservices architecture and Docker.

## Tech Stack

- **Backend:** Go 1.21
- **Database:** PostgreSQL 15
- **Frontend:** HTML/CSS/JavaScript
- **Containerization:** Docker & Docker Compose
- **CI/CD:** GitHub Actions

## Architecture

```
                    ┌─────────────┐
                    │   Gateway   │ :8080
                    └──────┬──────┘
         ┌─────────┬───────┼───────┬─────────┬─────────┐
    ┌────▼───┐ ┌───▼──┐ ┌──▼──┐ ┌──▼──┐ ┌────▼───┐ ┌───▼────┐
    │  User  │ │ Prod │ │Cart │ │Order│ │Payment │ │Notific │
    │ :8001  │ │:8002 │ │:8003│ │:8004│ │ :8005  │ │ :8006  │
    └────────┘ └──────┘ └─────┘ └─────┘ └────────┘ └────────┘
                           │
                    ┌──────▼──────┐
                    │  PostgreSQL │
                    └─────────────┘
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| Gateway | 8080 | API routing, rate limiting, CORS |
| User | 8001 | Authentication, profiles |
| Product | 8002 | Catalog, inventory |
| Cart | 8003 | Shopping cart management |
| Order | 8004 | Order processing |
| Payment | 8005 | Payment simulation |
| Notification | 8006 | Email/SMS alerts |

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21+ (for local development)

### Run with Docker

```bash
git clone https://github.com/joycezhou/go-ecommerce-microservices.git
cd go-ecommerce-microservices
docker-compose up --build
```

Access the app at `http://localhost:8080`

### Run Locally

```bash
# Start PostgreSQL
docker-compose up postgres -d

# Run services (each in separate terminal)
go run services/user/main.go
go run services/product/main.go
go run services/cart/main.go
go run services/order/main.go
go run services/payment/main.go
go run services/notification/main.go
go run services/gateway/main.go
```

## API Endpoints

### Auth
- `POST /api/register` - Register user
- `POST /api/login` - Login

### Products
- `GET /api/products` - List products
- `GET /api/products/{id}` - Get product
- `GET /api/categories` - List categories

### Cart
- `GET /api/cart/{user_id}` - Get cart
- `POST /api/cart/{user_id}/items` - Add item
- `PUT /api/cart/{user_id}/items/{item_id}` - Update quantity
- `DELETE /api/cart/{user_id}/items/{item_id}` - Remove item

### Orders
- `POST /api/orders` - Create order
- `GET /api/orders/user/{user_id}` - Get user orders
- `GET /api/orders/{id}` - Get order details

### Payments
- `POST /api/payments` - Process payment
- `GET /api/payments/{id}` - Get payment

### Health
- `GET /api/health` - All services health check

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_USER | postgres | Database user |
| DB_PASSWORD | postgres | Database password |
| JWT_SECRET | (generated) | JWT signing key |

## License

MIT
