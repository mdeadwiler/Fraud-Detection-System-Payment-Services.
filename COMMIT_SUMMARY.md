## Commit Summary

**feat: implement real-time fraud detection system**

Added complete fraud detection capability with rule-based evaluation engine, PostgreSQL persistence, Redis velocity caching, and REST API. Supports 6 rule types (velocity, amount, geographic, device, merchant, behavioral) with configurable thresholds and scoring strategies. Includes Docker setup, database migrations, and graceful degradation for standalone operation.

