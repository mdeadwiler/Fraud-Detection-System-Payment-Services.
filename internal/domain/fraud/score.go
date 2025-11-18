package fraud

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ScoreWeights defines how much each rule type contributes to final score
type ScoreWeights struct {
	Velocity decimal.Decimal `json:"velocity"`
	Amount decimal.Decimal `json:"amount"`
	Geographic decimal.Decimal `json:"geographic"`
	Device decimal.Decimal `json:"device"`
	Merchandise decimal.Decimal `json:"merchandise"`
	Behavior decimal.Decimal `json:"behavior"`
	MLModel decimal.Decimal `json:"ml_model"`
}