package ml

import (
	"context"
	"sync"

	"github.com/shopspring/decimal"

	"fraud-detecction-system/internal/domain/fraud"
)

// Predictor handles ML-based fraud prediction
type Predictor struct {
	featureExtractor *FeatureExtractor
	modelVersion     string
	enabled          bool
	mu               sync.RWMutex

	// In a real implementation, this would be a loaded ML model
	// For now, we use a rule-based heuristic that mimics ML behavior
	weights []float64
}

// NewPredictor creates a new ML predictor
func NewPredictor(extractor *FeatureExtractor, modelVersion string, enabled bool) *Predictor {
	return &Predictor{
		featureExtractor: extractor,
		modelVersion:     modelVersion,
		enabled:          enabled,
		// These weights simulate a trained model
		// In production, load actual model weights
		weights: defaultModelWeights(),
	}
}

// Predict returns a fraud probability score using ML
func (p *Predictor) Predict(ctx context.Context, evalCtx *fraud.RuleEvaluationContext) (*PredictionResult, error) {
	p.mu.RLock()
	enabled := p.enabled
	version := p.modelVersion
	p.mu.RUnlock()

	if !enabled {
		return &PredictionResult{
			Score:        decimal.Zero,
			Confidence:   decimal.Zero,
			ModelVersion: version,
			Enabled:      false,
		}, nil
	}

	// Extract features
	features := p.featureExtractor.Extract(ctx, evalCtx)
	vector := features.ToVector()

	// Calculate score using linear combination (simplified ML)
	// In production, this would be a proper ML model inference
	score := p.calculateScore(vector)

	// Calculate confidence based on feature completeness
	confidence := p.calculateConfidence(features)

	return &PredictionResult{
		Score:            decimal.NewFromFloat(score),
		Confidence:       decimal.NewFromFloat(confidence),
		ModelVersion:     version,
		Enabled:          true,
		FeatureVector:    vector,
		TopFeatures:      p.getTopContributors(vector),
	}, nil
}

// PredictionResult contains the ML prediction output
type PredictionResult struct {
	Score         decimal.Decimal   `json:"score"`          // 0.0 to 1.0 fraud probability
	Confidence    decimal.Decimal   `json:"confidence"`     // Model confidence
	ModelVersion  string            `json:"model_version"`
	Enabled       bool              `json:"enabled"`
	FeatureVector []float64         `json:"feature_vector,omitempty"`
	TopFeatures   map[string]float64 `json:"top_features,omitempty"`
}

// IsEnabled returns whether ML prediction is enabled
func (p *Predictor) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

// SetEnabled enables or disables ML prediction
func (p *Predictor) SetEnabled(enabled bool) {
	p.mu.Lock()
	p.enabled = enabled
	p.mu.Unlock()
}

// GetModelVersion returns the current model version
func (p *Predictor) GetModelVersion() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.modelVersion
}

func (p *Predictor) calculateScore(vector []float64) float64 {
	if len(vector) != len(p.weights) {
		return 0.0
	}

	// Linear combination with sigmoid activation
	sum := 0.0
	for i, v := range vector {
		sum += v * p.weights[i]
	}

	// Sigmoid to get probability
	return sigmoid(sum)
}

func (p *Predictor) calculateConfidence(features *Features) float64 {
	// Confidence based on data completeness
	confidence := 0.5 // Base confidence

	// More data = higher confidence
	if features.DeviceCount > 0 {
		confidence += 0.1
	}
	if features.AccountAgeDays > 30 {
		confidence += 0.1
	}
	if features.TxCountLastDay > 0 {
		confidence += 0.1
	}
	if features.AvgTransactionAmount > 0 {
		confidence += 0.1
	}
	if features.IsKnownLocation == 1.0 || features.IsKnownDevice == 1.0 {
		confidence += 0.1
	}

	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

func (p *Predictor) getTopContributors(vector []float64) map[string]float64 {
	featureNames := []string{
		"amount", "amount_log", "is_high_value",
		"hour_of_day", "day_of_week", "is_weekend", "is_night_time",
		"tx_count_hour", "tx_count_day", "tx_amount_hour", "tx_amount_day",
		"is_known_location", "is_cross_border", "is_blocked_country", "distance_from_last",
		"is_known_device", "is_trusted_device", "device_count",
		"account_age_days", "days_since_last_activity", "avg_tx_amount", "amount_deviation",
		"is_high_risk_merchant", "is_known_merchant",
	}

	contributions := make(map[string]float64)
	for i, v := range vector {
		if i < len(featureNames) && i < len(p.weights) {
			contrib := v * p.weights[i]
			if contrib > 0.05 || contrib < -0.05 { // Only significant contributions
				contributions[featureNames[i]] = contrib
			}
		}
	}

	return contributions
}

func sigmoid(x float64) float64 {
	if x > 10 {
		return 0.99999
	}
	if x < -10 {
		return 0.00001
	}
	return 1.0 / (1.0 + exp(-x))
}

func exp(x float64) float64 {
	// Simple exp approximation for small values
	if x == 0 {
		return 1.0
	}
	result := 1.0
	term := 1.0
	for i := 1; i <= 20; i++ {
		term *= x / float64(i)
		result += term
	}
	return result
}

// defaultModelWeights returns weights that simulate a trained model
// These weights emphasize known fraud indicators
func defaultModelWeights() []float64 {
	return []float64{
		0.001,  // amount - small positive
		0.05,   // amount_log
		0.3,    // is_high_value
		0.01,   // hour_of_day
		0.0,    // day_of_week
		0.05,   // is_weekend
		0.15,   // is_night_time
		0.2,    // tx_count_hour - velocity
		0.1,    // tx_count_day
		0.001,  // tx_amount_hour
		0.0005, // tx_amount_day
		-0.3,   // is_known_location - negative = reduces risk
		0.25,   // is_cross_border
		0.8,    // is_blocked_country - strong indicator
		0.001,  // distance_from_last
		-0.25,  // is_known_device - negative = reduces risk
		-0.35,  // is_trusted_device
		0.05,   // device_count
		-0.01,  // account_age_days - older is safer
		0.02,   // days_since_last_activity
		-0.001, // avg_tx_amount
		0.15,   // amount_deviation
		0.4,    // is_high_risk_merchant
		-0.2,   // is_known_merchant
	}
}

