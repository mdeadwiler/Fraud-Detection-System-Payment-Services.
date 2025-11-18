package fraud 

import ( 
	"context"
	"time"

	"github.com/google/uuid"
)

// DecisionRepository manages fraud decisions
type DecisionRepository interface {
	// Create stores for fraud decision
	Create(ctx context.Context, decision *FraudDecision) error

	// GetById retrieves a decision by ID
	GetByID(ctx context.Context, decision *FraudDecision) error

	// GetByTransactionID retrieves decision or a transaction
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
	GetByID(ctx context.Context, userID uuid.UUID) (*FraudCase, error)

	// Update updates an existing case
	Update(ctx context.Context, fraudCase *FraudCase) error

	// ListByStatus retrieves cases by status
	ListByStatus(ctx context.Context, status CaseStatus, limit, offset int) ([]*FraudCase, error)

	// ListByAssignee retrieves cases assigned to an investigator
	ListByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*FraudCase, error)

	// GetOpenCaseByUser checks if user has any open fraud cases
	GetOpenCaseByUser(ctx context.Context, userID uuid.UUID) ([]*FraudCase, error)
}

