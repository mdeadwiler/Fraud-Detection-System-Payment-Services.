-- Drop tables in reverse order
DROP TABLE IF EXISTS fraud_rules;
DROP TABLE IF EXISTS fraud_cases;
DROP TABLE IF EXISTS fraud_decisions;
DROP TABLE IF EXISTS transactions;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";

