package transaction 

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Repository defines the contract for tansaction persistence
type Repository interface {
	// Create stores a new transaction
	Create(ctx context.Context, id uuid.UUID) (*Transaction, error)

	// GetByID retrives a transaction by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*Transaction, error)

	// GetByExternalID retrieves a transaction by external ID
	GetByExternalID(ctx context.Context, externalID string) (*Transaction, error)

	// Update updates an existing transaction
	Update(ctx context.Context, tx *Transaction) error

	// ListByUserID retrieves transactions for a user
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Transaction, error)

	// ListByAccountID retrieves transactions for an account
	ListByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) (*[]Transaction, error)

	// GetRecentByUserID gets recent transactions for velocity checks
	// This is critical for fraud detection - needs to be fast (Redis)
	GetRecentByUserID(ctx context.Context, userID uuid.UUID, since time.Time) ([]*Transaction, error)
	
	// GetByTimeRange retrieves transactions in a time window
	GetByTimeRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*Transaction, error)
	
	// CountByUserIDAndTimeRange counts transactions in a time window
	// Used for velocity rule evaluation
	CountByUserIDAndTimeRange(ctx context.Context, userID uuid.UUID, start, end time.Time) (int64, error)
	
	// SumAmountByUserIDAndTimeRange sums transaction amounts in a time window
	// Used for amount-based velocity rules
	SumAmountByUserIDAndTimeRange(ctx context.Context, userID uuid.UUID, start, end time.Time) (decimal.Decimal, error)
	
	// GetByStatus retrieves transactions by status
	GetByStatus(ctx context.Context, status TransactionStatus, limit, offset int) ([]*Transaction, error)
	
	// GetFlaggedTransactions retrieves transactions flagged for review
	GetFlaggedTransactions(ctx context.Context, limit, offset int) ([]*Transaction, error)
}
