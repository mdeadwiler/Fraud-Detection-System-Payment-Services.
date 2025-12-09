package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"fraud-detecction-system/internal/domain/transaction"
)

// TransactionModel is the database model for transactions
type TransactionModel struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey"`
	ExternalID  string          `gorm:"type:varchar(100);index"`
	UserID      uuid.UUID       `gorm:"type:uuid;index;not null"`
	AccountID   uuid.UUID       `gorm:"type:uuid;index;not null"`
	Type        string          `gorm:"type:varchar(20);not null"`
	Status      string          `gorm:"type:varchar(20);index;not null"`
	Amount      decimal.Decimal `gorm:"type:decimal(15,2);not null"`
	Currency    string          `gorm:"type:varchar(3);not null"`
	Description string          `gorm:"type:text"`
	Location    string          `gorm:"type:jsonb"`
	Device      string          `gorm:"type:jsonb"`
	Merchant    string          `gorm:"type:jsonb"`
	Payment     string          `gorm:"type:jsonb"`
	Metadata    string          `gorm:"type:jsonb"`
	FraudScore  *decimal.Decimal `gorm:"type:decimal(5,4)"`
	RiskLevel   string          `gorm:"type:varchar(20)"`
	FraudReasons string         `gorm:"type:jsonb"`
	ReviewedBy  *uuid.UUID      `gorm:"type:uuid"`
	ReviewedAt  *time.Time
	CreatedAt   time.Time       `gorm:"not null"`
	ProcessedAt *time.Time
	UpdatedAt   time.Time       `gorm:"not null"`
}

// TableName returns the table name for transactions
func (TransactionModel) TableName() string {
	return "transactions"
}

// TransactionRepository implements transaction.Repository
type TransactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a new transaction repository
func NewTransactionRepository(client *Client) *TransactionRepository {
	return &TransactionRepository{db: client.DB()}
}

// Create stores a new transaction
func (r *TransactionRepository) Create(ctx context.Context, tx *transaction.Transaction) error {
	model := &TransactionModel{
		ID:          tx.ID,
		ExternalID:  tx.ExternalID,
		UserID:      tx.UserID,
		AccountID:   tx.AccountID,
		Type:        string(tx.Type),
		Status:      string(tx.Status),
		Amount:      tx.Amount,
		Currency:    string(tx.Currency),
		Description: tx.Description,
		FraudScore:  tx.FraudScore,
		RiskLevel:   tx.RiskLevel,
		CreatedAt:   tx.CreatedAt,
		ProcessedAt: tx.ProcessedAt,
		UpdatedAt:   tx.UpdatedAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

// GetByID retrieves a transaction by ID
func (r *TransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*transaction.Transaction, error) {
	var model TransactionModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, transaction.ErrTransactionNotFound
		}
		return nil, err
	}
	return modelToTransaction(&model), nil
}

// Update updates an existing transaction
func (r *TransactionRepository) Update(ctx context.Context, tx *transaction.Transaction) error {
	return r.db.WithContext(ctx).Model(&TransactionModel{}).
		Where("id = ?", tx.ID).
		Updates(map[string]interface{}{
			"status":       string(tx.Status),
			"fraud_score":  tx.FraudScore,
			"risk_level":   tx.RiskLevel,
			"reviewed_by":  tx.ReviewedBy,
			"reviewed_at":  tx.ReviewedAt,
			"processed_at": tx.ProcessedAt,
			"updated_at":   time.Now(),
		}).Error
}

// ListByUserID retrieves transactions for a user
func (r *TransactionRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*transaction.Transaction, error) {
	var models []TransactionModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error; err != nil {
		return nil, err
	}

	transactions := make([]*transaction.Transaction, len(models))
	for i, m := range models {
		transactions[i] = modelToTransaction(&m)
	}
	return transactions, nil
}

// ListPending retrieves pending transactions
func (r *TransactionRepository) ListPending(ctx context.Context, limit int) ([]*transaction.Transaction, error) {
	var models []TransactionModel
	if err := r.db.WithContext(ctx).
		Where("status = ?", "pending").
		Order("created_at ASC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}

	transactions := make([]*transaction.Transaction, len(models))
	for i, m := range models {
		transactions[i] = modelToTransaction(&m)
	}
	return transactions, nil
}

func modelToTransaction(m *TransactionModel) *transaction.Transaction {
	return &transaction.Transaction{
		ID:          m.ID,
		ExternalID:  m.ExternalID,
		UserID:      m.UserID,
		AccountID:   m.AccountID,
		Type:        transaction.TransactionType(m.Type),
		Status:      transaction.TransactionStatus(m.Status),
		Amount:      m.Amount,
		Currency:    transaction.Currency(m.Currency),
		Description: m.Description,
		FraudScore:  m.FraudScore,
		RiskLevel:   m.RiskLevel,
		ReviewedBy:  m.ReviewedBy,
		ReviewedAt:  m.ReviewedAt,
		CreatedAt:   m.CreatedAt,
		ProcessedAt: m.ProcessedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

