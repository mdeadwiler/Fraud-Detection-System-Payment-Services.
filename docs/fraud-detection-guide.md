# Fraud Detection Feature Guide

## Overview

The fraud detection system analyzes payment transactions in real-time and returns a decision based on configurable rules. Each transaction is evaluated against multiple rule types, scored, and classified into one of four decision categories.

## How It Works

1. Transaction data is submitted to the `/api/v1/fraud/analyze` endpoint
2. The rule engine evaluates all active rules against the transaction
3. Each fired rule contributes a score (0.0 to 1.0)
4. The highest score determines the final decision
5. Results are persisted and returned to the caller

## Getting Started

### Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ (for local development)
- Port 8080, 5432, and 6379 available

### Step 1: Start Infrastructure

```bash
cd /path/to/Fraud-Detection-System-Payment-Services
docker-compose -f docker-compose.dev.yml up -d
```

This starts:
- PostgreSQL on port 5432 (with schema and default rules)
- Redis on port 6379

Verify containers are running:
```bash
docker ps
```

### Step 2: Build the API

```bash
go build -o fraud-api ./cmd/api/main.go
```

### Step 3: Start the API

```bash
./fraud-api -config configs/config.yaml
```

Expected output:
```
Starting Fraud Detection API v1.0.0
Server will listen on 0.0.0.0:8080
Connected to PostgreSQL at localhost:5432
Connected to Redis at localhost:6379
HTTP server listening on 0.0.0.0:8080
```

### Step 4: Verify Installation

```bash
curl http://localhost:8080/ready
```

Expected response:
```json
{
  "status": "ready",
  "services": {
    "database": "healthy",
    "redis": "healthy"
  }
}
```

## Usage

### Analyzing a Transaction

```bash
curl -X POST http://localhost:8080/api/v1/fraud/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_id": "550e8400-e29b-41d4-a716-446655440001",
    "user_id": "550e8400-e29b-41d4-a716-446655440002",
    "account_id": "550e8400-e29b-41d4-a716-446655440003",
    "amount": "500.00",
    "currency": "USD",
    "location": {
      "country": "US",
      "city": "New York",
      "latitude": 40.7128,
      "longitude": -74.0060
    },
    "device": {
      "device_id": "device-123",
      "device_type": "mobile",
      "os": "iOS",
      "is_trusted_device": true
    },
    "merchant": {
      "merchant_id": "merchant-456",
      "merchant_name": "Amazon",
      "merchant_category": "5411",
      "country": "US",
      "is_high_risk": false
    }
  }'
```

### Response Format

```json
{
  "decision": "allow",
  "score": "0",
  "risk_level": "low",
  "confidence": "0.5",
  "rules_fired": [],
  "reasons": [],
  "latency_ms": 12,
  "should_block": false,
  "requires_review": false
}
```

### Decision Values

| Decision | Action Required |
|----------|-----------------|
| `allow` | Process transaction normally |
| `block` | Reject transaction |
| `review` | Queue for manual review |
| `challenge` | Request additional verification (2FA, etc.) |

## Default Rules

The system includes 6 pre-configured rules:

| Rule | Trigger | Decision |
|------|---------|----------|
| high_velocity | >5 transactions in 5 minutes | block |
| high_amount | Amount > $5,000 | review |
| blocked_countries | Transaction from KP, IR, or SY | block |
| new_device | Untrusted device | challenge |
| unusual_time | Transaction between 2-5 AM | review |
| high_risk_merchant | Gambling/crypto merchant (MCC 7995, 6051) | review |

## Viewing Active Rules

```bash
curl http://localhost:8080/api/v1/fraud/rules
```

## Creating Custom Rules

```bash
curl -X POST http://localhost:8080/api/v1/fraud/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "large_withdrawal",
    "description": "Flag withdrawals over $2000",
    "type": "amount",
    "severity": "high",
    "action": "review",
    "config": {
      "max_amount": "2000"
    }
  }'
```

## Standalone Mode

The system can run without PostgreSQL and Redis for testing:

```bash
./fraud-api
```

In standalone mode:
- Uses in-memory storage (data not persisted)
- Velocity checks are disabled
- Default rules are loaded from code

## Troubleshooting

### Port Already in Use

```bash
lsof -ti:8080 | xargs kill -9
```

### Database Connection Failed

Verify PostgreSQL is running:
```bash
docker exec fraud-postgres pg_isready -U fraud_user
```

### Redis Connection Failed

Verify Redis is running:
```bash
docker exec fraud-redis redis-cli ping
```

### No Rules Loading

Check database has rules:
```bash
docker exec fraud-postgres psql -U fraud_user -d fraud_detection -c "SELECT name FROM fraud_rules;"
```

## Stopping the System

```bash
# Stop API
pkill -f fraud-api

# Stop Docker containers
docker-compose -f docker-compose.dev.yml down
```

