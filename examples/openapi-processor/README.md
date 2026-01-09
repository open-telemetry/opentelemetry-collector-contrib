# OpenAPI Processor Example

This example demonstrates the **OpenAPI Processor** enriching traces with `url.template` and `peer.service` attributes based on OpenAPI specifications.

## Overview

The OpenAPI processor reads OpenAPI specifications and matches incoming trace spans against the defined paths. When a match is found, it adds:

- `url.template`: The OpenAPI path template (e.g., `/users/{userId}`)
- `peer.service`: The service name from the OpenAPI spec's `info.title` field

This is useful for:
- **Reducing cardinality**: Normalize high-cardinality URL paths for better metrics aggregation
- **Service identification**: Automatically identify which API a trace belongs to

## Architecture

```
                         ┌──────────────────┐
                         │   k6 Load Test   │
                         └────────┬─────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                            Order Service (8081)                              │
│                                                                              │
│  POST /orders ──────────────────────────▶ User Service (8082)                │
│       └─▶ Validates user                   GET /users/{userId}               │
│                                                                              │
│  POST /orders/{orderId}/items ──────────▶ Product Service (8083)             │
│       └─▶ Fetches product details          GET /products/{productId}         │
│                                                                              │
│  POST /orders/{orderId}/checkout ───────▶ Payment Service (8084)             │
│       └─▶ Processes payment                POST /payments                    │
└──────────────────────────────────────────────────────────────────────────────┘
                                  │
                                  │ OTLP Traces
                                  ▼
                         ┌──────────────────┐
                         │  OTel Collector  │
                         │  with OpenAPI    │
                         │   Processors     │
                         └────────┬─────────┘
                                  │
                                  ▼
                         ┌──────────────────┐
                         │   Grafana LGTM   │
                         │  (Loki, Grafana, │
                         │   Tempo, Mimir)  │
                         └──────────────────┘
```

## Services

### Order Service (port 8081)
Orchestrates the order flow and calls other services:
- `GET /health` - Health check
- `POST /orders` - Create a new order (calls User Service to validate user)
- `POST /orders/{orderId}/items` - Add item to order (calls Product Service to fetch product details)
- `POST /orders/{orderId}/checkout` - Checkout order (calls Payment Service to process payment)

### User Service (port 8082)
Manages user data:
- `GET /health` - Health check
- `GET /users/{userId}` - Get user by ID

### Product Service (port 8083)
Manages product catalog:
- `GET /health` - Health check
- `GET /products/{productId}` - Get product by ID

### Payment Service (port 8084)
Processes payments:
- `GET /health` - Health check
- `POST /payments` - Create a new payment

## Running the Example

### Start the services

```bash
cd examples/openapi-processor
docker-compose up -d
```

This will start:
- **otel-collector**: Custom build with OpenAPI processor
- **lgtm**: Grafana LGTM stack (Loki, Grafana, Tempo, Mimir) for observability
- **order-service**: Main orchestrator service
- **user-service**: User management
- **product-service**: Product catalog
- **payment-service**: Payment processing
- **k6**: Load generator that automatically runs the E2E flow

### Access the services

- **Grafana**: http://localhost:3000 (admin/admin)
- **Order Service**: http://localhost:8081
- **User Service**: http://localhost:8082
- **Product Service**: http://localhost:8083
- **Payment Service**: http://localhost:8084

### Generate traces manually

The k6 load test automatically generates traces, but you can also trigger the flow manually:

```bash
# Step 1: Create an order (validates user via user-service)
curl -X POST http://localhost:8081/orders \
  -H "Content-Type: application/json" \
  -d '{"userId": "user-1"}'

# Step 2: Add item to order (fetches product from product-service)
# Replace <orderId> with the ID from Step 1
curl -X POST http://localhost:8081/orders/<orderId>/items \
  -H "Content-Type: application/json" \
  -d '{"productId": "prod-1", "quantity": 1}'

# Step 3: Checkout order (processes payment via payment-service)
curl -X POST http://localhost:8081/orders/<orderId>/checkout
```

### View traces in Grafana

1. Open Grafana at http://localhost:3000 (login: admin/admin)
2. Go to **Explore**
3. Select **Tempo** as the data source
4. Search for traces by service name (e.g., `Order Service API`)

### Expected Results

In Grafana/Tempo, you should see traces with enriched attributes:

**Before OpenAPI Processor:**
```
http.url: http://user-service:8080/users/user-123
```

**After OpenAPI Processor:**
```
http.url: http://user-service:8080/users/user-123
url.template: /users/{userId}
peer.service: User Service API
```

## OpenAPI Processor Configuration

The collector configuration (`otel-collector-config.yaml`) shows how to configure multiple OpenAPI processors:

```yaml
processors:
  # OpenAPI processor for Order Service
  openapi/order-service:
    openapi_file: /etc/otel/openapi/order-service.yaml
    url_attribute: http.url
    url_template_attribute: url.template
    peer_service_attribute: peer.service
    use_server_url_matching: true

  # OpenAPI processor for User Service
  openapi/user-service:
    openapi_file: /etc/otel/openapi/user-service.yaml
    # ... same config ...

  # OpenAPI processor for Product Service
  openapi/product-service:
    openapi_file: /etc/otel/openapi/product-service.yaml
    # ... same config ...

  # OpenAPI processor for Payment Service
  openapi/payment-service:
    openapi_file: /etc/otel/openapi/payment-service.yaml
    # ... same config ...
```

Each processor:
1. Loads its OpenAPI specification at startup
2. Uses `use_server_url_matching: true` to only process traces that match its server URL
3. Adds `url.template` and `peer.service` attributes to matching spans
4. Passes through non-matching spans unchanged

## Span Metrics

This example also demonstrates the **spanmetrics connector** which generates metrics from trace spans. The metrics include:
- `http.server.request.duration` - Request duration histogram
- Request count with dimensions for `http.route`, `peer.service`, `url.template`, etc.

## Stop the services

```bash
docker-compose down -v
```