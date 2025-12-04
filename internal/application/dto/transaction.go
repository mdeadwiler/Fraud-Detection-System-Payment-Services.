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
}

