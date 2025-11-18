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

// DefaultScoreWeights provides a balanced starting point
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		Velocity:   decimal.NewFromFloat(0.25),
		Amount:     decimal.NewFromFloat(0.15),
		Geographic: decimal.NewFromFloat(0.20),
		Device:     decimal.NewFromFloat(0.15),
		Merchant:   decimal.NewFromFloat(0.10),
		Behavioral: decimal.NewFromFloat(0.10),
		MLModel:    decimal.NewFromFloat(0.05),
	}
}

// ScoringStrategy defines how multiple rule results are combined
type ScoringStrategy string

const (
	StrategyWeightedAverage ScoringStrategy = "weighted_average"
	StrategyMaxScore        ScoringStrategy = "max_score"
	StrategyBayesian        ScoringStrategy = "bayesian"
)

// FraudScorer calculates final fraud scores from rule results