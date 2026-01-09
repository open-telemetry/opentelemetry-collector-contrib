import http from 'k6/http';
import { check, sleep } from 'k6';
import { randomItem } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// Configuration
const ORDER_SERVICE = __ENV.ORDER_SERVICE_URL || 'http://order-service:8080';

export const options = {
  stages: [
    { duration: '1s', target: 100 },   // Ramp up
    { duration: '10m', target: 500 },    // Steady load
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    http_req_failed: ['rate<0.1'],
  },
};

// Test data
const userIds = ['user-1', 'user-2'];
const productIds = ['prod-1', 'prod-2', 'prod-3'];

// Helper to make JSON POST requests
function jsonPost(url, body) {
  return http.post(url, JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
  });
}

// Single E2E Flow: Create Order -> Add Item -> Checkout
// This flow goes through all 4 services:
// 1. order-service (POST /orders) -> user-service (GET /users/:userId)
// 2. order-service (POST /orders/:id/items) -> product-service (GET /products/:productId)
// 3. order-service (POST /orders/:id/checkout) -> payment-service (POST /payments)
export default function() {
  const userId = randomItem(userIds);
  const productId = randomItem(productIds);
  
  // Step 1: Create order (order-service calls user-service)
  let res = jsonPost(`${ORDER_SERVICE}/orders`, { userId });
  const orderCreated = check(res, { 
    'create order status 201': (r) => r.status === 201 
  });
  
  if (!orderCreated) {
    console.error('Failed to create order:', res.status, res.body);
    sleep(1);
    return;
  }
  
  const order = JSON.parse(res.body);
  const orderId = order.id;
  
  // Step 2: Add item to order (order-service calls product-service)
  res = jsonPost(`${ORDER_SERVICE}/orders/${orderId}/items`, {
    productId,
    quantity: 1,
  });
  check(res, { 'add item status 201': (r) => r.status === 201 });
  
  // Step 3: Checkout order (order-service calls payment-service)
  res = jsonPost(`${ORDER_SERVICE}/orders/${orderId}/checkout`, {});
  check(res, { 
    'checkout status 200 or 402': (r) => r.status === 200 || r.status === 402 
  });
  
  sleep(1);
}
