# Fraud Detection System for Payment Services

Real-time fraud detection engine for payment transaction analysis. Evaluates transactions against configurable rules, calculates risk scores, and returns actionable decisions (allow, block, review, challenge).

## Architecture

```
cmd/api/                    Application entry point
internal/
  domain/fraud/             Business logic and entities
  application/fraud/        Use cases
  infrastructure/
    database/postgres/      PostgreSQL repositories
    cache/redis/            Velocity and device caching
    rules/                  Rule evaluation engine
    ml/                     Feature extraction and prediction
  interfaces/http/          REST API handlers
```

## Requirements

- Go 1.21+
- PostgreSQL 15+
- Redis 7+

## Quick Start

```bash
# Start dependencies
docker-compose -f docker-compose.dev.yml up -d

# Build
go build -o fraud-api ./cmd/api/main.go

# Run
./fraud-api -config configs/config.yaml
```

## API

### Analyze Transaction
```bash
POST /api/v1/fraud/analyze
```

### Batch Analysis
```bash
POST /api/v1/fraud/analyze/batch
```

### Rules Management
```bash
GET  /api/v1/fraud/rules
POST /api/v1/fraud/rules
```

### Case Management
```bash
GET /api/v1/fraud/cases
PUT /api/v1/fraud/cases/{id}
```

## Configuration

Environment variables override `configs/config.yaml`:

| Variable | Description | Default |
|----------|-------------|---------|
| `FRAUD_DB_HOST` | PostgreSQL host | localhost |
| `FRAUD_DB_PORT` | PostgreSQL port | 5432 |
| `FRAUD_REDIS_HOST` | Redis host | localhost |
| `FRAUD_REDIS_PORT` | Redis port | 6379 |
| `FRAUD_SERVER_PORT` | API port | 8080 |

## Rule Types

| Type | Description |
|------|-------------|
| velocity | Transaction frequency limits |
| amount | Transaction value thresholds |
| geographic | Location-based restrictions |
| device | Device trust verification |
| merchant | Merchant risk assessment |
| behavioral | User pattern analysis |

## Decision Thresholds

| Score | Decision |
|-------|----------|
| >= 0.80 | Block |
| >= 0.60 | Review |
| >= 0.40 | Challenge |
| < 0.40 | Allow |

## License

See LICENSE file.
