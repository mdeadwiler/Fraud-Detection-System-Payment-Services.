package fraud

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fraud-detecction-system/internal/domain/fraud"
	"fraud-detecction-system/internal/domain/transaction"
	"fraud-detecction-system/internal/infrastructure/cache/redis"
	"fraud-detecction-system/internal/infrastructure/ml"
	"fraud-detecction-system/internal/infrastructure/rules"
)

// DetectFraudInput contains the input for fraud detection
type DetectFraudInput struct {
	TransactionID uuid.UUID
	UserID        uuid.UUID
	AccountID     uuid.UUID
	Amount        decimal.Decimal
	Currency      string
	Timestamp     time.Time

	// Optional context data
	Location *fraud.GeoLocation
	Device   *fraud.DeviceInfo
	Merchant *fraud.MerchantInfo
	Payment  *fraud.PaymentMethod
}

// DetectFraudOutput contains the fraud detection result
type DetectFraudOutput struct {
	Decision        fraud.DecisionType  `json:"decision"`
	Score           decimal.Decimal     `json:"score"`
	RiskLevel       fraud.RiskLevel     `json:"risk_level"`
	Confidence      decimal.Decimal     `json:"confidence"`
	RulesFired      []string            `json:"rules_fired"`
	Reasons         []string            `json:"reasons"`
	ModelVersion    string              `json:"model_version,omitempty"`
	LatencyMs       int64               `json:"latency_ms"`
	ShouldBlock     bool                `json:"should_block"`
	RequiresReview  bool                `json:"requires_review"`
}

// DetectFraudUseCase handles fraud detection for transactions
type DetectFraudUseCase struct {
	fraudService  *fraud.Service
	ruleEngine    *rules.Engine
	mlPredictor   *ml.Predictor
	velocityCache *redis.VelocityCache
	deviceCache   *redis.DeviceCache
	locationCache *redis.LocationCache

	// Config
	analysisTimeout time.Duration
}

// NewDetectFraudUseCase creates a new detect fraud use case
func NewDetectFraudUseCase(
	fraudService *fraud.Service,
	ruleEngine *rules.Engine,
	mlPredictor *ml.Predictor,
	velocityCache *redis.VelocityCache,
	deviceCache *redis.DeviceCache,
	locationCache *redis.LocationCache,
	analysisTimeout time.Duration,
) *DetectFraudUseCase {
	return &DetectFraudUseCase{
		fraudService:    fraudService,
		ruleEngine:      ruleEngine,
		mlPredictor:     mlPredictor,
		velocityCache:   velocityCache,
		deviceCache:     deviceCache,
		locationCache:   locationCache,
		analysisTimeout: analysisTimeout,
	}
}

// Execute performs fraud detection on a transaction
func (uc *DetectFraudUseCase) Execute(ctx context.Context, input DetectFraudInput) (*DetectFraudOutput, error) {
	startTime := time.Now()

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, uc.analysisTimeout)
	defer cancel()

	// Build evaluation context
	evalCtx := &fraud.RuleEvaluationContext{
		TransactionID: input.TransactionID,
		UserID:        input.UserID,
		AccountID:     input.AccountID,
		Amount:        input.Amount,
		Currency:      input.Currency,
		Timestamp:     input.Timestamp,
		Location:      input.Location,
		Device:        input.Device,
		Merchant:      input.Merchant,
		Payment:       input.Payment,
	}

	// Enrich context with historical data
	if err := uc.enrichContext(ctx, evalCtx); err != nil {
		// Log error but continue - we can still evaluate with available data
	}

	// Run fraud analysis through the service
	decision, err := uc.fraudService.AnalyzeTransaction(ctx, evalCtx)
	if err != nil {
		return nil, fmt.Errorf("fraud analysis failed: %w", err)
	}

	// Record transaction for velocity tracking (async, don't wait)
	go func() {
		bgCtx := context.Background()
		if uc.velocityCache != nil {
			uc.velocityCache.RecordTransaction(bgCtx, input.UserID, input.TransactionID, input.Amount, input.Timestamp)
		}
		if uc.deviceCache != nil && input.Device != nil {
			uc.deviceCache.RecordDeviceUsage(bgCtx, input.UserID, input.Device.DeviceID)
		}
		if uc.locationCache != nil && input.Location != nil {
			uc.locationCache.RecordLocation(bgCtx, input.UserID, input.Location.Country, input.Location.City)
		}
	}()

	// Build output
	output := &DetectFraudOutput{
		Decision:       decision.Decision,
		Score:          decision.Score,
		RiskLevel:      decision.RiskLevel,
		Confidence:     decision.Confidence,
		RulesFired:     decision.RulesFired,
		Reasons:        decision.Reasons,
		ModelVersion:   decision.ModelVersion,
		LatencyMs:      time.Since(startTime).Milliseconds(),
		ShouldBlock:    decision.ShouldBlock(),
		RequiresReview: decision.RequiresReview(),
	}

	return output, nil
}

// enrichContext adds historical data to the evaluation context
func (uc *DetectFraudUseCase) enrichContext(ctx context.Context, evalCtx *fraud.RuleEvaluationContext) error {
	// Get recent transactions from cache
	if uc.velocityCache != nil {
		records, err := uc.velocityCache.GetRecentTransactions(ctx, evalCtx.UserID, 24*time.Hour)
		if err == nil {
			evalCtx.RecentTransactions = make([]fraud.TransactionSummary, len(records))
			for i, r := range records {
				evalCtx.RecentTransactions[i] = fraud.TransactionSummary{
					ID:        r.TransactionID,
					Amount:    r.Amount,
					Timestamp: r.Timestamp,
				}
			}
		}
	}

	// Build basic user profile from available data
	if evalCtx.UserProfile == nil {
		evalCtx.UserProfile = &fraud.UserProfile{
			UserID:         evalCtx.UserID,
			AccountAge:     24 * time.Hour * 30, // Default to 30 days if unknown
			LastActivityAt: time.Now().Add(-24 * time.Hour),
		}
	}

	return nil
}

// AnalyzeTransactionRequest is the API request structure
type AnalyzeTransactionRequest struct {
	TransactionID string  `json:"transaction_id" validate:"required,uuid"`
	UserID        string  `json:"user_id" validate:"required,uuid"`
	AccountID     string  `json:"account_id" validate:"required,uuid"`
	Amount        string  `json:"amount" validate:"required"`
	Currency      string  `json:"currency" validate:"required,len=3"`

	// Optional
	Location *LocationRequest `json:"location,omitempty"`
	Device   *DeviceRequest   `json:"device,omitempty"`
	Merchant *MerchantRequest `json:"merchant,omitempty"`
	Payment  *PaymentRequest  `json:"payment,omitempty"`
}

// LocationRequest represents location data in API request
type LocationRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Region    string  `json:"region,omitempty"`
	IPAddress string  `json:"ip_address,omitempty"`
}

// DeviceRequest represents device data in API request
type DeviceRequest struct {
	DeviceID        string `json:"device_id"`
	DeviceType      string `json:"device_type"`
	OS              string `json:"os"`
	Browser         string `json:"browser,omitempty"`
	UserAgent       string `json:"user_agent,omitempty"`
	IsTrustedDevice bool   `json:"is_trusted_device"`
}

// MerchantRequest represents merchant data in API request
type MerchantRequest struct {
	MerchantID       string `json:"merchant_id"`
	MerchantName     string `json:"merchant_name"`
	MerchantCategory string `json:"merchant_category"` // MCC code
	Country          string `json:"country"`
	IsHighRisk       bool   `json:"is_high_risk"`
}

// PaymentRequest represents payment method data in API request
type PaymentRequest struct {
	Type           string `json:"type"` // card, bank_account, wallet
	Last4          string `json:"last4"`
	Network        string `json:"network"`
	BankID         string `json:"bank_id,omitempty"`
	IssuingCountry string `json:"issuing_country"`
}

// ToInput converts the API request to use case input
func (r *AnalyzeTransactionRequest) ToInput() (*DetectFraudInput, error) {
	txID, err := uuid.Parse(r.TransactionID)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction_id: %w", err)
	}

	userID, err := uuid.Parse(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	accountID, err := uuid.Parse(r.AccountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account_id: %w", err)
	}

	amount, err := decimal.NewFromString(r.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	input := &DetectFraudInput{
		TransactionID: txID,
		UserID:        userID,
		AccountID:     accountID,
		Amount:        amount,
		Currency:      r.Currency,
		Timestamp:     time.Now(),
	}

	// Convert optional fields
	if r.Location != nil {
		input.Location = &fraud.GeoLocation{
			Latitude:  r.Location.Latitude,
			Longitude: r.Location.Longitude,
			Country:   r.Location.Country,
			City:      r.Location.City,
			Region:    r.Location.Region,
			IPAddress: r.Location.IPAddress,
		}
	}

	if r.Device != nil {
		input.Device = &fraud.DeviceInfo{
			DeviceID:        r.Device.DeviceID,
			DeviceType:      r.Device.DeviceType,
			OS:              r.Device.OS,
			Browser:         r.Device.Browser,
			UserAgent:       r.Device.UserAgent,
			IsTrustedDevice: r.Device.IsTrustedDevice,
			LastSeenAt:      time.Now(),
		}
	}

	if r.Merchant != nil {
		input.Merchant = &fraud.MerchantInfo{
			MerchantID:       r.Merchant.MerchantID,
			MerchantName:     r.Merchant.MerchantName,
			MerchantCategory: r.Merchant.MerchantCategory,
			Country:          r.Merchant.Country,
			IsHighRisk:       r.Merchant.IsHighRisk,
		}
	}

	if r.Payment != nil {
		input.Payment = &fraud.PaymentMethod{
			Type:           r.Payment.Type,
			Last4:          r.Payment.Last4,
			Network:        r.Payment.Network,
			BankID:         r.Payment.BankID,
			IssuingCountry: r.Payment.IssuingCountry,
		}
	}

	return input, nil
}

// BatchAnalyzeInput contains multiple transactions to analyze
type BatchAnalyzeInput struct {
	Transactions []DetectFraudInput `json:"transactions"`
}

// BatchAnalyzeOutput contains results for multiple transactions
type BatchAnalyzeOutput struct {
	Results []DetectFraudOutput `json:"results"`
	Summary BatchSummary        `json:"summary"`
}

// BatchSummary summarizes batch analysis results
type BatchSummary struct {
	Total     int   `json:"total"`
	Allowed   int   `json:"allowed"`
	Blocked   int   `json:"blocked"`
	Review    int   `json:"review"`
	Challenge int   `json:"challenge"`
	AvgLatencyMs int64 `json:"avg_latency_ms"`
}

// ExecuteBatch performs fraud detection on multiple transactions
func (uc *DetectFraudUseCase) ExecuteBatch(ctx context.Context, input BatchAnalyzeInput) (*BatchAnalyzeOutput, error) {
	results := make([]DetectFraudOutput, len(input.Transactions))
	summary := BatchSummary{Total: len(input.Transactions)}
	var totalLatency int64

	for i, tx := range input.Transactions {
		result, err := uc.Execute(ctx, tx)
		if err != nil {
			// Record error but continue with other transactions
			results[i] = DetectFraudOutput{
				Decision:  fraud.DecisionReview,
				RiskLevel: fraud.RiskLevelHigh,
				Reasons:   []string{"Analysis error: " + err.Error()},
			}
			summary.Review++
			continue
		}

		results[i] = *result
		totalLatency += result.LatencyMs

		switch result.Decision {
		case fraud.DecisionAllow:
			summary.Allowed++
		case fraud.DecisionBlock:
			summary.Blocked++
		case fraud.DecisionReview:
			summary.Review++
		case fraud.DecisionChallenge:
			summary.Challenge++
		}
	}

	if summary.Total > 0 {
		summary.AvgLatencyMs = totalLatency / int64(summary.Total)
	}

	return &BatchAnalyzeOutput{
		Results: results,
		Summary: summary,
	}, nil
}
