package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/shopspring/decimal"
)

// Create trasaction request represents a request to create a new transaction for detection

type CreateTreansactionRequests struct {
ExternalID string `json:"external_id" validate:"required"`
UserID uuid.UUID `json:"user_id" validate:"required"`
AccountID uuid.UUID `json:"account_id" validate:"required"`
Type string `json:"type" validate:"required,oneof=purchase refund withdraw deposit trasfer"`
Amount decimal.Decimal `json:"amount" validate:"required"`
Currency string `json:"currency" validate:"required,len=3"`
Description string `json:"description"`

// Fraud Detection Results
FraudScore *decimal.Decimal `json:"fraud_score,omitempty"`
RiskLevel string `json:"risk_level,omitempty"`
FraudReasons string `json:"fraud_reasons,omitempty"`
RequiredReview bool `json:"required_review"`

//Perfomance metrics
ProcessingTimeMs int64 `json:"processing_time_ms"`

CreatedAt time.Time `json:"created_at"`
ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// GeoLocationDTO represents geographic location data
type GeoLocation struct {
	Latitude float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country string `json:"country"`
	City string `json:"city"`
	Region string `json:"region"`
	IPAddress string `json:"ip_address"`
}

// MerchantDTO represents merchant information
type MerchantDTO struct {
	MerchantId string `json:"type"`
	Name string `json:"name"`
	Category string `json:"category"`
	Country string `json:"country"`
	RiskScore string `json:"risk_score,omitempty"`
}

// PaymentDTO for payment information
type PaymentDTO struct {
	Type string `json:"type"` // car, bank_transfer, wallet
	Last4 string `json:"last4,omitempty"`
	CardType string `json:"card_type,omitempty"`
	BankID string `json:"bank_id,omitempty"`
	IssuingCountry string `json:"issuing_country,omitempty"`
}

