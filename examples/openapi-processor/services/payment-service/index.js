require('./tracing');

const express = require('express');

const app = express();
app.use(express.json());

const PORT = process.env.PORT || 8080;
const APP_NAME = process.env.APP_NAME || 'payment-service';

// In-memory store
const payments = new Map();
let paymentIdCounter = 1;

function simulateWork(minMs = 10, maxMs = 50) {
  const duration = Math.floor(Math.random() * (maxMs - minMs + 1)) + minMs;
  return new Promise(resolve => setTimeout(resolve, duration));
}

function simulatePaymentProcessing() {
  // Simulate payment processing with occasional failures
  return new Promise((resolve, reject) => {
    setTimeout(() => {
      const success = Math.random() > 0.1; // 90% success rate
      if (success) {
        resolve({ success: true, transactionId: `txn-${Date.now()}` });
      } else {
        reject(new Error('Payment declined'));
      }
    }, Math.random() * 100 + 50);
  });
}

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'healthy', service: APP_NAME });
});

// Create payment - called by order-service during checkout
app.post('/payments', async (req, res) => {
  await simulateWork();
  
  const paymentId = `pay-${paymentIdCounter++}`;
  
  const payment = {
    id: paymentId,
    orderId: req.body.orderId,
    userId: req.body.userId,
    amount: req.body.amount,
    currency: req.body.currency || 'USD',
    status: 'processing',
    createdAt: new Date().toISOString(),
  };
  
  try {
    const result = await simulatePaymentProcessing();
    payment.transactionId = result.transactionId;
    payment.status = 'completed';
    
    payments.set(paymentId, payment);
    
    res.status(201).json(payment);
  } catch (error) {
    payment.status = 'failed';
    payment.error = error.message;
    payments.set(paymentId, payment);
    
    res.status(402).json(payment);
  }
});

app.listen(PORT, () => {
  console.log(`[${APP_NAME}] Server running on port ${PORT}`);
});
