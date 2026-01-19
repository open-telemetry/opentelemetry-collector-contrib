require('./tracing');

const express = require('express');
const axios = require('axios');

const app = express();
app.use(express.json());

const PORT = process.env.PORT || 8080;
const APP_NAME = process.env.APP_NAME || 'order-service';
const USER_SERVICE = process.env.USER_SERVICE;
const PRODUCT_SERVICE = process.env.PRODUCT_SERVICE;
const PAYMENT_SERVICE = process.env.PAYMENT_SERVICE;

// In-memory store
const orders = new Map();
let orderIdCounter = 1;

function simulateWork(minMs = 10, maxMs = 50) {
  const duration = Math.floor(Math.random() * (maxMs - minMs + 1)) + minMs;
  return new Promise(resolve => setTimeout(resolve, duration));
}

async function callService(url, method = 'GET', data = null) {
  try {
    const config = {
      method,
      url,
      headers: { 'Content-Type': 'application/json' },
    };
    if (data) config.data = data;
    const response = await axios(config);
    return response.data;
  } catch (error) {
    console.error(`Failed to call ${url}:`, error.message);
    return { error: error.message };
  }
}

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'healthy', service: APP_NAME });
});

// Create order - calls user-service to validate user
app.post('/orders', async (req, res) => {
  await simulateWork(20, 80);
  
  const orderId = `order-${orderIdCounter++}`;
  const order = {
    id: orderId,
    userId: req.body.userId,
    items: [],
    status: 'pending',
    total: 0,
    createdAt: new Date().toISOString(),
  };
  
  // Validate user exists by calling user-service
  if (USER_SERVICE && req.body.userId) {
    const user = await callService(`${USER_SERVICE}/users/${req.body.userId}`);
    if (user.error) {
      return res.status(400).json({ error: 'Invalid user' });
    }
    order.userName = user.name;
  }
  
  orders.set(orderId, order);
  
  res.status(201).json(order);
});

// Add item to order - calls product-service to get product details
app.post('/orders/:orderId/items', async (req, res) => {
  await simulateWork(30, 100);
  
  const order = orders.get(req.params.orderId);
  if (!order) {
    return res.status(404).json({ error: 'Order not found' });
  }
  
  // Get product details from product-service
  let productPrice = req.body.price || 10;
  let productName = 'Unknown Product';
  if (PRODUCT_SERVICE && req.body.productId) {
    const product = await callService(`${PRODUCT_SERVICE}/products/${req.body.productId}`);
    if (!product.error) {
      productPrice = product.price || productPrice;
      productName = product.name || productName;
    }
  }
  
  const item = {
    id: `item-${Date.now()}`,
    productId: req.body.productId,
    productName,
    quantity: req.body.quantity || 1,
    price: productPrice,
  };
  
  order.items.push(item);
  order.total = order.items.reduce((sum, i) => sum + (i.price * i.quantity), 0);
  
  res.status(201).json(item);
});

// Complete order - calls payment-service to process payment
app.post('/orders/:orderId/checkout', async (req, res) => {
  await simulateWork(20, 60);
  
  const order = orders.get(req.params.orderId);
  if (!order) {
    return res.status(404).json({ error: 'Order not found' });
  }
  
  if (order.status !== 'pending') {
    return res.status(400).json({ error: 'Order already processed' });
  }
  
  // Process payment via payment-service
  if (PAYMENT_SERVICE) {
    const payment = await callService(`${PAYMENT_SERVICE}/payments`, 'POST', {
      orderId: order.id,
      amount: order.total,
      userId: order.userId,
    });
    
    if (payment.error || payment.status === 'failed') {
      order.status = 'payment_failed';
      return res.status(402).json({ error: 'Payment failed', order });
    }
    
    order.paymentId = payment.id;
    order.transactionId = payment.transactionId;
  }
  
  order.status = 'completed';
  order.completedAt = new Date().toISOString();
  
  res.json(order);
});

app.listen(PORT, () => {
  console.log(`[${APP_NAME}] Server running on port ${PORT}`);
});
