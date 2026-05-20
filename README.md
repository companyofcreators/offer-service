# Offer Service

Manages the immutable negotiation (trading) process between masters and customers.

## Business Rules

1. One master can have only ONE pending offer per order at a time
2. Customer can counter any pending offer (creates event, not new offer)
3. When an offer is accepted, ALL other pending offers for that order are auto-rejected
4. Master cannot update offer price - must withdraw and send new
5. Only the offer's creator (master) can withdraw it
6. Only the order's customer can accept/reject/counter

## API Endpoints

```
POST   /internal/offers                  # Send offer (master)
POST   /internal/offers/{id}/withdraw    # Withdraw offer (master)
POST   /internal/offers/{id}/accept      # Accept offer (customer)
POST   /internal/offers/{id}/reject      # Reject offer (customer)
POST   /internal/offers/{id}/counter     # Counter-propose price (customer)
GET    /internal/offers/{id}             # Get offer details
GET    /internal/offers                  # List offers (query: order_id, master_id, status)
GET    /internal/offers/{id}/history     # Get negotiation history for an offer
GET    /internal/orders/{id}/history     # Get full negotiation history for an order
GET    /internal/health                  # Health check
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| HTTP_ADDRESS | :8084 | HTTP server address |
| DB_DSN | postgres://postgres:postgres@localhost:5432/offers?sslmode=disable | PostgreSQL connection string |
| KAFKA_BROKERS | localhost:9092 | Comma-separated Kafka broker addresses |
| ORDER_SERVICE_URL | http://localhost:8082 | Order service URL for validating customer ownership |
| LOG_LEVEL | info | Logging level (debug, info, warn, error) |

## Running

```bash
# Copy environment file
cp .env.example .env

# Run database migrations
# Use your preferred migration tool (e.g., golang-migrate)

# Build and run
go run ./cmd/api/main.go
```

## Docker

```bash
docker build -t offer-service .
docker run -p 8084:8084 --env-file .env offer-service
```
