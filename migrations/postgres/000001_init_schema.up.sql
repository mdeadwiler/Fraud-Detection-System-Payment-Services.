-- Fraud Detection System Schema

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    external_id VARCHAR(100),
    user_id UUID NOT NULL,
    account_id UUID NOT NULL,
    type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    amount DECIMAL(15,2) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    description TEXT,
    location JSONB,
    device JSONB,
    merchant JSONB,
    payment JSONB,
    metadata JSONB,
    fraud_score DECIMAL(5,4),
    risk_level VARCHAR(20),
    fraud_reasons JSONB,
    reviewed_by UUID,
    reviewed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_account_id ON transactions(account_id);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_created_at ON transactions(created_at);

-- Fraud Decisions table
CREATE TABLE IF NOT EXISTS fraud_decisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL,
    user_id UUID NOT NULL,
    decision VARCHAR(20) NOT NULL,
    score DECIMAL(5,4) NOT NULL,
    risk_level VARCHAR(20) NOT NULL,
    confidence DECIMAL(5,4),
    rules_fired JSONB,
    reasons JSONB,
    model_version VARCHAR(50),
    processed_at TIMESTAMP NOT NULL,
    latency_ms BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fraud_decisions_transaction_id ON fraud_decisions(transaction_id);
CREATE INDEX idx_fraud_decisions_user_id ON fraud_decisions(user_id);
CREATE INDEX idx_fraud_decisions_decision ON fraud_decisions(decision);
CREATE INDEX idx_fraud_decisions_created_at ON fraud_decisions(created_at);

-- Fraud Cases table
CREATE TABLE IF NOT EXISTS fraud_cases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_ids JSONB NOT NULL,
    user_id UUID NOT NULL,
    account_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    risk_level VARCHAR(20) NOT NULL,
    total_amount DECIMAL(15,2),
    currency VARCHAR(3),
    assigned_to UUID,
    description TEXT,
    notes JSONB,
    evidence JSONB,
    resolution TEXT,
    resolved_by UUID,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fraud_cases_user_id ON fraud_cases(user_id);
CREATE INDEX idx_fraud_cases_account_id ON fraud_cases(account_id);
CREATE INDEX idx_fraud_cases_status ON fraud_cases(status);
CREATE INDEX idx_fraud_cases_assigned_to ON fraud_cases(assigned_to);

-- Fraud Rules table
CREATE TABLE IF NOT EXISTS fraud_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    type VARCHAR(20) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    action VARCHAR(20) NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    version INT NOT NULL DEFAULT 1,
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    effective_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP
);

CREATE INDEX idx_fraud_rules_type ON fraud_rules(type);
CREATE INDEX idx_fraud_rules_enabled ON fraud_rules(enabled);

-- Insert default rules
INSERT INTO fraud_rules (id, name, description, type, severity, action, config, created_by) VALUES
    (uuid_generate_v4(), 'high_velocity', 'Block if more than 5 transactions in 5 minutes', 'velocity', 'high', 'block', 
     '{"max_transactions": 5, "window_minutes": 5}', '00000000-0000-0000-0000-000000000000'),
    (uuid_generate_v4(), 'high_amount', 'Review transactions over $5000', 'amount', 'medium', 'review', 
     '{"max_amount": "5000", "deviation_factor": 5}', '00000000-0000-0000-0000-000000000000'),
    (uuid_generate_v4(), 'blocked_countries', 'Block transactions from high-risk countries', 'geographic', 'critical', 'block', 
     '{"blocked_countries": ["KP", "IR", "SY"], "require_consistent": true}', '00000000-0000-0000-0000-000000000000'),
    (uuid_generate_v4(), 'new_device', 'Challenge transactions from new devices', 'device', 'medium', 'challenge', 
     '{"require_trusted_device": true, "max_devices_per_user": 5}', '00000000-0000-0000-0000-000000000000'),
    (uuid_generate_v4(), 'unusual_time', 'Review transactions at unusual times', 'behavioral', 'low', 'review', 
     '{}', '00000000-0000-0000-0000-000000000000'),
    (uuid_generate_v4(), 'high_risk_merchant', 'Review transactions with high-risk merchants', 'merchant', 'medium', 'review', 
     '{"high_risk_mcc_codes": ["7995", "7801", "5967", "6051"]}', '00000000-0000-0000-0000-000000000000');

