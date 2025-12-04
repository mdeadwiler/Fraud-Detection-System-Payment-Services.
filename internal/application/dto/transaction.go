package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/shopspring/decimal"
)

// Create trasaction request represents a request to create a new transaction for detection

type CreateTreansactionRequests struct {
ExternalID string `json:"external_id:"required`
UserID uuid.UUID `json:"user_id" validate: "required`
AccountID uuid.UUID `json:"account_id" validate:"required`

}
