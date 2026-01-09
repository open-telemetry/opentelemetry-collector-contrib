require('./tracing');

const express = require('express');

const app = express();
app.use(express.json());

const PORT = process.env.PORT || 8080;
const APP_NAME = process.env.APP_NAME || 'product-service';

// In-memory store with seed data
const products = new Map();
products.set('prod-1', {
  id: 'prod-1',
  name: 'Wireless Headphones',
  description: 'High-quality wireless headphones',
  price: 99.99,
  inventory: 50,
});
products.set('prod-2', {
  id: 'prod-2',
  name: 'Cotton T-Shirt',
  description: 'Comfortable cotton t-shirt',
  price: 29.99,
  inventory: 100,
});
products.set('prod-3', {
  id: 'prod-3',
  name: 'JavaScript Guide',
  description: 'Complete JavaScript programming guide',
  price: 49.99,
  inventory: 25,
});

function simulateWork(minMs = 5, maxMs = 30) {
  const duration = Math.floor(Math.random() * (maxMs - minMs + 1)) + minMs;
  return new Promise(resolve => setTimeout(resolve, duration));
}

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'healthy', service: APP_NAME });
});

// Get product by ID - called by order-service when adding items
app.get('/products/:productId', async (req, res) => {
  await simulateWork();
  const product = products.get(req.params.productId);
  if (!product) {
    return res.status(404).json({ error: 'Product not found' });
  }
  res.json(product);
});

app.listen(PORT, () => {
  console.log(`[${APP_NAME}] Server running on port ${PORT}`);
});
