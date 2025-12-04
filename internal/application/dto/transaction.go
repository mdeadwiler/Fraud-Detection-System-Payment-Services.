package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/shopspring/decimal"
)

// CreateTreansactionRequests represents a request to create a new transaction for detection
type CreateTreansactionRequests struct {
	ExternalID  string          `json:"external_id" validate:"required"`
	UserID      uuid.UUID       `json:"user_id" validate:"required"`
	AccountID   uuid.UUID       `json:"account_id" validate:"required"`
	Type        string          `json:"type" validate:"required,oneof=purchase refund withdraw deposit transfer"`
	Amount      decimal.Decimal `json:"amount" validate:"required"`
	Currency    string          `json:"currency" validate:"required,len=3"`
	Description string          `json:"description"`

	// Context for fraud detection
	Location *GeoLocation   `json:"location,omitempty"`
	Device   *DeviceInfoDTO `json:"device,omitempty"`
	Merchant *MerchantDTO   `json:"merchant,omitempty"`
	Payment  *PaymentDTO    `json:"payment,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TransactionResponse represents the transaction result with fraud analysis
type TransactionResponse struct {
	ID          uuid.UUID       `json:"id"`
	ExternalID  string          `json:"external_id"`
	UserID      uuid.UUID       `json:"user_id"`
	AccountID   uuid.UUID       `json:"account_id"`
	Type        string          `json:"type"`
	Status      string          `json:"status"`
	Amount      decimal.Decimal `json:"amount"`
	Currency    string          `json:"currency"`
	Description string          `json:"description"`

	// Fraud detection results
	FraudScore     *decimal.Decimal `json:"fraud_score,omitempty"`
	RiskLevel      string           `json:"risk_level,omitempty"`
	FraudReasons   []string         `json:"fraud_reasons,omitempty"`
	RequiresReview bool             `json:"requires_review"`

	// Performance metrics
	ProcessingTimeMs int64 `json:"processing_time_ms"`

	CreatedAt   time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// GeoLocation represents geographic location data
type GeoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Region    string  `json:"region,omitempty"`
	IPAddress string  `json:"ip_address"`
}

// DeviceInfoDTO represents device information
type DeviceInfoDTO struct {
	DeviceID      string `json:"device_id"`
	Fingerprint   string `json:"fingerprint"`
	Type          string `json:"type"`
	OS            string `json:"os"`
	Browser       string `json:"browser,omitempty"`
	UserAgent     string `json:"user_agent"`
	TrustedDevice bool   `json:"trusted_device"`
}

// MerchantDTO represents merchant information
type MerchantDTO struct {
	MerchantID string `json:"merchant_id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Country    string `json:"country"`
	RiskScore  string `json:"risk_score,omitempty"`
}

// PaymentDTO for payment information
type PaymentDTO struct {
	Type string `json:"type"` // car, bank_transfer, wallet
	Last4 string `json:"last4,omitempty"`
	CardType string `json:"card_type,omitempty"`
	BankID string `json:"bank_id,omitempty"`
	IssuingCountry string `json:"issuing_country,omitempty"`
}

