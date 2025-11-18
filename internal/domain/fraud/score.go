package fraud

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ScoreWeights defines how much each rule type contributes to final score
type ScoreWeights struct {
	Velocity   decimal.Decimal `json:"velocity"`
	Amount     decimal.Decimal `json:"amount"`
	Geographic decimal.Decimal `json:"geographic"`
	Device     decimal.Decimal `json:"device"`
	Merchant   decimal.Decimal `json:"merchant"`
	Behavioral decimal.Decimal `json:"behavioral"`
	MLModel    decimal.Decimal `json:"ml_model"`
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
type FraudScorer interface {
	// CalculateScore computes the final fraud score
	CalculateScore(ctx context.Context, results []RuleResult, weights ScoreWeights) (decimal.Decimal, error)

	// DetermineDecision decides the action based on score
	DetermineDecision(ctx context.Context, score decimal.Decimal, riskLevel RiskLevel) (DecisionType, error)

	// GetRiskLevel converts a score to a risk level
	GetRiskLevel(score decimal.Decimal) RiskLevel
}

// ScoreCalculationResult contains the detailed scoring breakdown
type ScoreCalculationResult struct {
	FinalScore        decimal.Decimal            `json:"final_score"`
	RiskLevel         RiskLevel                  `json:"risk_level"`
	Decision          DecisionType               `json:"decision"`
	RuleContributions map[string]decimal.Decimal `json:"rule_contributions"`
	Strategy          ScoringStrategy            `json:"strategy"`
	CalculatedAt      time.Time                  `json:"calculated_at"`
}

// DecisionThresholds defines score thresholds for decisions
type DecisionThresholds struct {
	BlockThreshold     decimal.Decimal // Above this = block
	ReviewThreshold    decimal.Decimal // Above this = review
	ChallengeThreshold decimal.Decimal // Above this = challenge
	// Below challenge threshold = allow
}

// DefaultDecisionThresholds provides reasonable defaults
func DefaultDecisionThresholds() DecisionThresholds {
	return DecisionThresholds{
		BlockThreshold:     decimal.NewFromFloat(0.80),
		ReviewThreshold:    decimal.NewFromFloat(0.60),
		ChallengeThreshold: decimal.NewFromFloat(0.40),
	}
}

// AggregateRuleResults combines multiple rule results
func AggregateRuleResults(results []RuleResult, weights ScoreWeights, strategy ScoringStrategy) (*ScoreCalculationResult, error) {
	switch strategy {
	case StrategyWeightedAverage:
		return aggregateWeightedAverage(results, weights)
	case StrategyMaxScore:
		return aggregateMaxScore(results)
	case StrategyBayesian:
		return aggregateBayesian(results, weights)
	default:
		return aggregateWeightedAverage(results, weights)
	}
}

func aggregateWeightedAverage(results []RuleResult, weights ScoreWeights) (*ScoreCalculationResult, error) {
	totalScore := decimal.Zero
	contributions := make(map[string]decimal.Decimal)

	for _, result := range results {
		if !result.Fired {
			continue
		}

		// Get weight for this rule type (simplified - in production map RuleID to type)
		weight := getWeightForRule(result.RuleName, weights)
		contribution := result.Score.Mul(weight)

		totalScore = totalScore.Add(contribution)
		contributions[result.RuleName] = contribution
	}

	// Normalize to 0-1 range
	if totalScore.GreaterThan(decimal.NewFromInt(1)) {
		totalScore = decimal.NewFromInt(1)
	}

	return &ScoreCalculationResult{
		FinalScore:        totalScore,
		RiskLevel:         getRiskLevel(totalScore),
		RuleContributions: contributions,
		Strategy:          StrategyWeightedAverage,
		CalculatedAt:      time.Now(),
	}, nil
}

func aggregateMaxScore(results []RuleResult) (*ScoreCalculationResult, error) {
	maxScore := decimal.Zero
	contributions := make(map[string]decimal.Decimal)

	for _, result := range results {
		if result.Fired && result.Score.GreaterThan(maxScore) {
			maxScore = result.Score
		}
		if result.Fired {
			contributions[result.RuleName] = result.Score
		}
	}

	return &ScoreCalculationResult{
		FinalScore:        maxScore,
		RiskLevel:         getRiskLevel(maxScore),
		RuleContributions: contributions,
		Strategy:          StrategyMaxScore,
		CalculatedAt:      time.Now(),
	}, nil
}

func aggregateBayesian(results []RuleResult, weights ScoreWeights) (*ScoreCalculationResult, error) {
	// Bayesian combination: P(fraud | evidence)
	// Using naive Bayes assumption for simplicity
	// In production, train on historical data for better priors

	priorFraudRate := decimal.NewFromFloat(0.01) // 1% base fraud rate
	posteriorOdds := priorFraudRate.Div(decimal.NewFromInt(1).Sub(priorFraudRate))

	for _, result := range results {
		if result.Fired {
			// Likelihood ratio = P(evidence | fraud) / P(evidence | not fraud)
			// Simplified: use rule score as proxy for likelihood
			likelihoodRatio := result.Score.Div(decimal.NewFromInt(1).Sub(result.Score))
			posteriorOdds = posteriorOdds.Mul(likelihoodRatio)
		}
	}

	// Convert odds back to probability
	finalProb := posteriorOdds.Div(decimal.NewFromInt(1).Add(posteriorOdds))

	contributions := make(map[string]decimal.Decimal)
	for _, result := range results {
		if result.Fired {
			contributions[result.RuleName] = result.Score
		}
	}

	return &ScoreCalculationResult{
		FinalScore:        finalProb,
		RiskLevel:         getRiskLevel(finalProb),
		RuleContributions: contributions,
		Strategy:          StrategyBayesian,
		CalculatedAt:      time.Now(),
	}, nil
}

func getRiskLevel(score decimal.Decimal) RiskLevel {
	scoreFloat := score.InexactFloat64()
	switch {
	case scoreFloat >= 0.80:
		return RiskLevelCritical
	case scoreFloat >= 0.60:
		return RiskLevelHigh
	case scoreFloat >= 0.30:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

func getWeightForRule(ruleName string, weights ScoreWeights) decimal.Decimal {
	// In production, maintain a mapping of rule names to types
	// This is simplified for demonstration
	return decimal.NewFromFloat(0.15)
}
