package transaction

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TransactionStatus represents the current state of a transaction
type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusApproved  TransactionStatus = "approved"
	StatusDeclined  TransactionStatus = "declined"
	StatusFlagged   TransactionStatus = "flagged"
	StatusReviewing TransactionStatus = "reviewing"
)

// TransactionType categorizes the type of transaction
type TransactionType string

const (
	TypePurchase  TransactionType = "purchase"
	TypeRefund    TransactionType = "refund"
	TypeWithdraw  TransactionType = "withdraw"
	TypeDeposit   TransactionType = "deposit"
	TypeTransfer  TransactionType = "transfer"
)

// Currency represents supported currency codes
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
)

// GeoLocation represents the geographic location of a transaction
// This is critical for fraud detection - comparing user's typical location
// against transaction origin helps identify account takeover attacks
type GeoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Region    string  `json:"region"`
	IPAddress string  `json:"ip_address"`
}

// DeviceInfo captures device fingerprint for fraud detection
// Multiple transactions from different devices for same account is suspicious
type DeviceInfo struct {
	DeviceID          string `json:"device_id"`
	DeviceType        string `json:"device_type"`        // mobile, desktop, tablet
	OS                string `json:"os"`                 // iOS, Android, Windows, etc
	Browser           string `json:"browser"`            // Chrome, Safari, etc
	UserAgent         string `json:"user_agent"`
	IsTrustedDevice   bool   `json:"is_trusted_device"`
	LastSeenAt        time.Time `json:"last_seen_at"`
}

// MerchantInfo contains merchant details for the transaction
// High-risk merchants or new merchants get additional scrutiny
type MerchantInfo struct {
	MerchantID       string `json:"merchant_id"`
	MerchantName     string `json:"merchant_name"`
	MerchantCategory string `json:"merchant_category"` // MCC code
	Country          string `json:"country"`
	IsHighRisk       bool   `json:"is_high_risk"`
}

// PaymentMethod represents how the transaction is being paid
type PaymentMethod struct {
	Type           string `json:"type"`            // card, bank_account, wallet
	Last4          string `json:"last4"`           // Last 4 digits for security
	Network        string `json:"network"`         // Visa, Mastercard, ACH, etc
	BankID         string `json:"bank_id"`         // Which bank this transaction is from
	IssuingCountry string `json:"issuing_country"`
}

// Transaction represents a financial transaction in the system
// This is our core domain entity that flows through the entire fraud detection pipeline
type Transaction struct {
	// Identity
	ID            uuid.UUID         `json:"id"`
	ExternalID    string            `json:"external_id"`    // ID from the originating bank/system

	// User Information
	UserID        uuid.UUID         `json:"user_id"`
	AccountID     uuid.UUID         `json:"account_id"`

	// Transaction Details
	Type          TransactionType   `json:"type"`
	Status        TransactionStatus `json:"status"`
	Amount        decimal.Decimal   `json:"amount"`          // Using decimal for financial precision
	Currency      Currency          `json:"currency"`
	Description   string            `json:"description"`

	// Context for Fraud Detection
	Location      *GeoLocation      `json:"location,omitempty"`
	Device        *DeviceInfo       `json:"device,omitempty"`
	Merchant      *MerchantInfo     `json:"merchant,omitempty"`
	Payment       *PaymentMethod    `json:"payment,omitempty"`

	// Metadata
	Metadata      map[string]string `json:"metadata,omitempty"` // Flexible key-value pairs

	// Timestamps - Critical for velocity checks
	CreatedAt     time.Time         `json:"created_at"`
	ProcessedAt   *time.Time        `json:"processed_at,omitempty"`
	UpdatedAt     time.Time         `json:"updated_at"`

	// Fraud Detection Fields
	FraudScore    *decimal.Decimal  `json:"fraud_score,omitempty"`     // 0.0 to 1.0 probability
	RiskLevel     string            `json:"risk_level,omitempty"`      // low, medium, high, critical
	FraudReasons  []string          `json:"fraud_reasons,omitempty"`   // Why was this flagged
	ReviewedBy    *uuid.UUID        `json:"reviewed_by,omitempty"`     // Who manually reviewed
	ReviewedAt    *time.Time        `json:"reviewed_at,omitempty"`
}

// NewTransaction creates a new transaction with required fields
// This enforces that critical fields are always present
func NewTransaction(userID, accountID uuid.UUID, txType TransactionType, amount decimal.Decimal, currency Currency) *Transaction {
	now := time.Now()
	return &Transaction{
		ID:        uuid.New(),
		UserID:    userID,
		AccountID: accountID,
		Type:      txType,
		Status:    StatusPending,
		Amount:    amount,
		Currency:  currency,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]string),
	}
}

// IsApproved checks if the transaction has been approved
func (t *Transaction) IsApproved() bool {
	return t.Status == StatusApproved
}

// IsDeclined checks if the transaction has been declined
func (t *Transaction) IsDeclined() bool {
	return t.Status == StatusDeclined
}

// IsFlagged checks if the transaction is flagged for review
func (t *Transaction) IsFlagged() bool {
	return t.Status == StatusFlagged || t.Status == StatusReviewing
}

// Approve marks the transaction as approved
func (t *Transaction) Approve() error {
	if t.Status != StatusPending && t.Status != StatusReviewing {
		return ErrInvalidStatusTransition
	}
	t.Status = StatusApproved
	now := time.Now()
	t.ProcessedAt = &now
	t.UpdatedAt = now
	return nil
}

// Decline marks the transaction as declined with a reason
func (t *Transaction) Decline(reasons []string) error {
	if t.Status != StatusPending && t.Status != StatusReviewing {
		return ErrInvalidStatusTransition
	}
	t.Status = StatusDeclined
	t.FraudReasons = reasons
	now := time.Now()
	t.ProcessedAt = &now
	t.UpdatedAt = now
	return nil
}

// FlagForReview marks the transaction for manual review
func (t *Transaction) FlagForReview(reasons []string, score decimal.Decimal) error {
	if t.Status != StatusPending {
		return ErrInvalidStatusTransition
	}
	t.Status = StatusFlagged
	t.FraudReasons = reasons
	t.FraudScore = &score
	t.UpdatedAt = time.Now()
	return nil
}

// MarkUnderReview indicates a human is reviewing this transaction
func (t *Transaction) MarkUnderReview(reviewerID uuid.UUID) error {
	if t.Status != StatusFlagged {
		return ErrInvalidStatusTransition
	}
	t.Status = StatusReviewing
	t.ReviewedBy = &reviewerID
	now := time.Now()
	t.ReviewedAt = &now
	t.UpdatedAt = now
	return nil
}

// SetFraudScore updates the fraud score for this transaction
func (t *Transaction) SetFraudScore(score decimal.Decimal, riskLevel string) {
	t.FraudScore = &score
	t.RiskLevel = riskLevel
	t.UpdatedAt = time.Now()
}

// AddFraudReason appends a fraud detection reason
func (t *Transaction) AddFraudReason(reason string) {
	t.FraudReasons = append(t.FraudReasons, reason)
	t.UpdatedAt = time.Now()
}

// IsHighValue determines if this is a high-value transaction
// High-value transactions get additional scrutiny
func (t *Transaction) IsHighValue(threshold decimal.Decimal) bool {
	return t.Amount.GreaterThan(threshold)
}

// IsCrossBorder checks if transaction crosses international borders
// Cross-border transactions have higher fraud risk
func (t *Transaction) IsCrossBorder() bool {
	if t.Location == nil || t.Payment == nil {
		return false
	}
	return t.Location.Country != t.Payment.IssuingCountry
}

// GetAge returns how long ago the transaction was created
func (t *Transaction) GetAge() time.Duration {
	return time.Since(t.CreatedAt)
}

// Validate performs basic validation on the transaction
func (t *Transaction) Validate() error {
	if t.ID == uuid.Nil {
		return ErrInvalidTransactionID
	}
	if t.UserID == uuid.Nil {
		return ErrInvalidUserID
	}
	if t.AccountID == uuid.Nil {
		return ErrInvalidAccountID
	}
	if t.Amount.IsNegative() {
		return ErrNegativeAmount
	}
	if t.Amount.IsZero() {
		return ErrZeroAmount
	}
	if t.Currency == "" {
		return ErrMissingCurrency
	}
	return nil
}
