package fraud

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DecisionRepository manages fraud decisions
type DecisionRepository interface {
	// Create stores a fraud decision
	Create(ctx context.Context, decision *FraudDecision) error

	// GetByID retrieves a decision by ID
	GetByID(ctx context.Context, id uuid.UUID) (*FraudDecision, error)

	// GetByTransactionID retrieves decision for a transaction
	GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*FraudDecision, error)

	// ListByUserID gets fraud decisions for a user
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*FraudDecision, error)

	// GetBlockedCount counts how many times a user has been blocked
	GetBlockedCount(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error)
}

// CaseRepository manages fraud investigation cases
type CaseRepository interface {
	// Create stores a new fraud case
	Create(ctx context.Context, fraudCase *FraudCase) error

	// GetByID retrieves a case by ID
	GetByID(ctx context.Context, id uuid.UUID) (*FraudCase, error)

	// Update updates an existing case
	Update(ctx context.Context, fraudCase *FraudCase) error

	// ListByStatus retrieves cases by status
	ListByStatus(ctx context.Context, status CaseStatus, limit, offset int) ([]*FraudCase, error)

	// ListByAssignee retrieves cases assigned to an investigator
	ListByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*FraudCase, error)

	// GetOpenCasesByUser checks if user has any open fraud cases
	GetOpenCasesByUser(ctx context.Context, userID uuid.UUID) ([]*FraudCase, error)
}

// RuleRepository manages fraud detection rules
type RuleRepository interface {
	// Create adds a new rule
	Create(ctx context.Context, rule *Rule) error

	// GetByID retrieves a rule by ID
	GetByID(ctx context.Context, id uuid.UUID) (*Rule, error)

	// Update updates an existing rule
	Update(ctx context.Context, rule *Rule) error

	// ListActive retrieves all enabled rules
	ListActive(ctx context.Context) ([]*Rule, error)

	// ListByType retrieves rules of a specific type
	ListByType(ctx context.Context, ruleType RuleType) ([]*Rule, error)

	// Disable disables a rule
	Disable(ctx context.Context, ruleID uuid.UUID) error

	// GetVersion retrieves a specific version of a rule
	GetVersion(ctx context.Context, ruleID uuid.UUID, version int) (*Rule, error)
}
