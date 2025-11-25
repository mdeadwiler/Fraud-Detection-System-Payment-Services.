package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Service handles transaction business logic
// This orchestrates transaction processing and state management
type Service struct {
	repo Repository

	// Configuration
	maxTransactionAmount decimal.Decimal
	minTransactionAmount decimal.Decimal
}

// NewService creates a new transaction service
func NewService(repo Repository) *Service {
	return &Service{
		repo:                 repo,
		maxTransactionAmount: decimal.NewFromInt(1000000), // $1M default max
		minTransactionAmount: decimal.NewFromFloat(0.01),  // $0.01 default min
	}
}

// CreateTransaction creates and persists a new transaction
func (s *Service) CreateTransaction(ctx context.Context, tx *Transaction) error {
	// Validate transaction
	if err := s.validateTransaction(tx); err != nil {
		return err
	}

	// Set initial state
	tx.Status = StatusPending
	tx.CreatedAt = time.Now()
	tx.UpdatedAt = time.Now()

	// Persist
	return s.repo.Create(ctx, tx)
}

// GetTransaction retrieves a transaction by ID
func (s *Service) GetTransaction(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	return s.repo.GetByID(ctx, id)
}

// GetTransactionByExternalID retrieves a transaction by external ID
func (s *Service) GetTransactionByExternalID(ctx context.Context, externalID string) (*Transaction, error) {
	return s.repo.GetByExternalID(ctx, externalID)
}

// ApproveTransaction approves a transaction
func (s *Service) ApproveTransaction(ctx context.Context, txID uuid.UUID) error {
	tx, err := s.repo.GetByID(ctx, txID)
	if err != nil {
		return err
	}

	if err := tx.Approve(); err != nil {
		return err
	}

	return s.repo.Update(ctx, tx)
}

// DeclineTransaction declines a transaction with reasons
func (s *Service) DeclineTransaction(ctx context.Context, txID uuid.UUID, reasons []string) error {
	tx, err := s.repo.GetByID(ctx, txID)
	if err != nil {
		return err
	}

	if err := tx.Decline(reasons); err != nil {
		return err
	}

	return s.repo.Update(ctx, tx)
}

// FlagForReview flags a transaction for manual review
func (s *Service) FlagForReview(ctx context.Context, txID uuid.UUID, reasons []string, score decimal.Decimal) error {
	tx, err := s.repo.GetByID(ctx, txID)
	if err != nil {
		return err
	}

	if err := tx.FlagForReview(reasons, score); err != nil {
		return err
	}

	return s.repo.Update(ctx, tx)
}

// UpdateFraudScore updates the fraud score for a transaction
func (s *Service) UpdateFraudScore(ctx context.Context, txID uuid.UUID, score decimal.Decimal, riskLevel string) error {
	tx, err := s.repo.GetByID(ctx, txID)
	if err != nil {
		return err
	}

	tx.SetFraudScore(score, riskLevel)
	return s.repo.Update(ctx, tx)
}

// ListUserTransactions retrieves transactions for a user
func (s *Service) ListUserTransactions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Transaction, error) {
	return s.repo.ListByUserID(ctx, userID, limit, offset)
}

// ListAccountTransactions retrieves transactions for an account
func (s *Service) ListAccountTransactions(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*Transaction, error) {
	return s.repo.ListByAccountID(ctx, accountID, limit, offset)
}

// GetRecentTransactions gets recent transactions for a user (for velocity checks)
func (s *Service) GetRecentTransactions(ctx context.Context, userID uuid.UUID, since time.Time) ([]*Transaction, error) {
	return s.repo.GetRecentByUserID(ctx, userID, since)
}

// GetTransactionsByTimeRange retrieves transactions in a time window
func (s *Service) GetTransactionsByTimeRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*Transaction, error) {
	return s.repo.GetByTimeRange(ctx, userID, start, end)
}

// CountTransactionsInWindow counts transactions in a time window (velocity check)
func (s *Service) CountTransactionsInWindow(ctx context.Context, userID uuid.UUID, windowDuration time.Duration) (int64, error) {
	start := time.Now().Add(-windowDuration)
	end := time.Now()
	return s.repo.CountByUserIDAndTimeRange(ctx, userID, start, end)
}

// SumTransactionAmountsInWindow sums transaction amounts in a time window
func (s *Service) SumTransactionAmountsInWindow(ctx context.Context, userID uuid.UUID, windowDuration time.Duration) (decimal.Decimal, error) {
	start := time.Now().Add(-windowDuration)
	end := time.Now()
	return s.repo.SumAmountByUserIDAndTimeRange(ctx, userID, start, end)
}

// CheckVelocity performs velocity checks on a user's transaction pattern
// Returns true if velocity thresholds are exceeded
func (s *Service) CheckVelocity(ctx context.Context, userID uuid.UUID, amount decimal.Decimal) (*VelocityCheckResult, error) {
	result := &VelocityCheckResult{
		UserID:     userID,
		CheckedAt:  time.Now(),
		Violations: make([]string, 0),
	}

	// Check 1: Transaction count in last 5 minutes
	count5min, err := s.CountTransactionsInWindow(ctx, userID, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	result.TransactionsLast5Min = count5min
	if count5min > 5 {
		result.Violations = append(result.Violations, fmt.Sprintf("Too many transactions in 5 minutes: %d (max 5)", count5min))
		result.VelocityExceeded = true
	}

	// Check 2: Transaction count in last hour
	count1hour, err := s.CountTransactionsInWindow(ctx, userID, 1*time.Hour)
	if err != nil {
		return nil, err
	}
	result.TransactionsLastHour = count1hour
	if count1hour > 20 {
		result.Violations = append(result.Violations, fmt.Sprintf("Too many transactions in 1 hour: %d (max 20)", count1hour))
		result.VelocityExceeded = true
	}

	// Check 3: Total amount in last hour
	sum1hour, err := s.SumTransactionAmountsInWindow(ctx, userID, 1*time.Hour)
	if err != nil {
		return nil, err
	}
	result.AmountLastHour = sum1hour
	maxHourlyAmount := decimal.NewFromInt(10000) // $10,000
	if sum1hour.GreaterThan(maxHourlyAmount) {
		result.Violations = append(result.Violations, fmt.Sprintf("Amount limit exceeded in 1 hour: %s (max %s)", sum1hour, maxHourlyAmount))
		result.VelocityExceeded = true
	}

	// Check 4: Total amount in last 24 hours
	sum24hours, err := s.SumTransactionAmountsInWindow(ctx, userID, 24*time.Hour)
	if err != nil {
		return nil, err
	}
	result.AmountLast24Hours = sum24hours
	maxDailyAmount := decimal.NewFromInt(50000) // $50,000
	if sum24hours.GreaterThan(maxDailyAmount) {
		result.Violations = append(result.Violations, fmt.Sprintf("Amount limit exceeded in 24 hours: %s (max %s)", sum24hours, maxDailyAmount))
		result.VelocityExceeded = true
	}

	return result, nil
}

// GetFlaggedTransactions retrieves all transactions flagged for review
func (s *Service) GetFlaggedTransactions(ctx context.Context, limit, offset int) ([]*Transaction, error) {
	return s.repo.GetFlaggedTransactions(ctx, limit, offset)
}

// GetTransactionsByStatus retrieves transactions by status
func (s *Service) GetTransactionsByStatus(ctx context.Context, status TransactionStatus, limit, offset int) ([]*Transaction, error) {
	return s.repo.GetByStatus(ctx, status, limit, offset)
}

// VelocityCheckResult contains the result of velocity checks
type VelocityCheckResult struct {
	UserID                uuid.UUID       `json:"user_id"`
	VelocityExceeded      bool            `json:"velocity_exceeded"`
	TransactionsLast5Min  int64           `json:"transactions_last_5min"`
	TransactionsLastHour  int64           `json:"transactions_last_hour"`
	AmountLastHour        decimal.Decimal `json:"amount_last_hour"`
	AmountLast24Hours     decimal.Decimal `json:"amount_last_24hours"`
	Violations            []string        `json:"violations"`
	CheckedAt             time.Time       `json:"checked_at"`
}

// Private validation methods

func (s *Service) validateTransaction(tx *Transaction) error {
	if tx == nil {
		return ErrInvalidTransaction
	}

	// Validate required fields
	if tx.UserID == uuid.Nil {
		return ErrInvalidTransaction
	}
	if tx.AccountID == uuid.Nil {
		return ErrInvalidTransaction
	}

	// Validate amount
	if tx.Amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}
	if tx.Amount.LessThan(s.minTransactionAmount) {
		return ErrAmountTooSmall
	}
	if tx.Amount.GreaterThan(s.maxTransactionAmount) {
		return ErrAmountTooLarge
	}

	// Validate currency
	if tx.Currency == "" {
		return ErrInvalidCurrency
	}

	// Validate transaction type
	validTypes := map[TransactionType]bool{
		TypePurchase: true,
		TypeRefund:   true,
		TypeWithdraw: true,
		TypeDeposit:  true,
		TypeTransfer: true,
	}
	if !validTypes[tx.Type] {
		return ErrInvalidTransactionType
	}

	return nil
}

// SetMaxTransactionAmount sets the maximum allowed transaction amount
func (s *Service) SetMaxTransactionAmount(amount decimal.Decimal) {
	s.maxTransactionAmount = amount
}

// SetMinTransactionAmount sets the minimum allowed transaction amount
func (s *Service) SetMinTransactionAmount(amount decimal.Decimal) {
	s.minTransactionAmount = amount
}