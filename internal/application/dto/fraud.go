package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/shopspring/decimal"
)

// FraudAnalysisResult represents the complete fraud analysis outcome
type FraudAnalysisResult struct {
	DecisionID    uuid.UUID       `json:"decision_id"`
	TransactionID uuid.UUID       `json:"transaction_id"`
	UserID        uuid.UUID       `json:"user_id"`

	// Decision
	Decision      string          `json:"decision"` // allow, block, review, challenge
	Score         decimal.Decimal `json:"score"`
	RiskLevel     string          `json:"risk_level"`
	Confidence    decimal.Decimal `json:"confidence"`

	// Explanation
	RulesFired    []string        `json:"rules_fired"`
	Reasons       []string        `json:"reasons"`

	// Case information (if created)
	CaseID        *uuid.UUID      `json:"case_id,omitempty"`
	CaseCreated   bool            `json:"case_created"`

	// Performance
	LatencyMs     int64           `json:"latency_ms"`
	ProcessedAt   time.Time       `json:"processed_at"`
}

// FraudeCaseResponses represents a fraud investigation case 
type FraudCaseResponses struct {
	ID uuid.UUID `json:"id"`
	TransactionIds []uuid.UUID `json:"transaction_ids"`
	UserID uuid.UUID `json:"user_id"`
	AccountID uuid.UUID `json:"account_id"`
	Status string `json:"status"`
	RiskLevel string `json:"risk_level"`
	TotalAmount decimal.Decimal `json:"total_amount"`
	Currency string `json:"currency"`
	AssignTo *uuid.UUID `json:"assign_to,omitempty"`
	Description string `json:"description"`
	Resolution string `json:"resolution"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}