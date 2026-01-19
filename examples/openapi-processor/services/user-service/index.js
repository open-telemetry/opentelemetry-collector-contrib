require('./tracing');

const express = require('express');

const app = express();
app.use(express.json());

const PORT = process.env.PORT || 8080;
const APP_NAME = process.env.APP_NAME || 'user-service';

// In-memory store with seed data
const users = new Map();
users.set('user-1', {
  id: 'user-1',
  name: 'John Doe',
  email: 'john@example.com',
  createdAt: new Date().toISOString(),
});
users.set('user-2', {
  id: 'user-2',
  name: 'Jane Smith',
  email: 'jane@example.com',
  createdAt: new Date().toISOString(),
});

function simulateWork(minMs = 5, maxMs = 30) {
  const duration = Math.floor(Math.random() * (maxMs - minMs + 1)) + minMs;
  return new Promise(resolve => setTimeout(resolve, duration));
}

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'healthy', service: APP_NAME });
});

// Get user by ID - called by order-service during order creation
app.get('/users/:userId', async (req, res) => {
  await simulateWork();
  const user = users.get(req.params.userId);
  if (!user) {
    return res.status(404).json({ error: 'User not found' });
  }
  res.json(user);
});

app.listen(PORT, () => {
  console.log(`[${APP_NAME}] Server running on port ${PORT}`);
});
