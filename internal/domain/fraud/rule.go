package fraud

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// RuleType categorizes the type of fraud rule
type RuleType string

const (
	RuleTypeVelocity   RuleType = "velocity"   // Transaction frequency
	RuleTypeAmount     RuleType = "amount"     // Transaction amount threshold
	RuleTypeGeographic RuleType = "geographic" // Location-based
	RuleTypeDevice     RuleType = "device"     // Device fingerprinting
	RuleTypeMerchant   RuleType = "merchant"   // Merchant risk
	RuleTypeBehavioral RuleType = "behavioral" // User behavior patterns
)

// RuleSeverity indicates how serious a rule violation is
type RuleSeverity string

const (
	SeverityLow      RuleSeverity = "low"
	SeverityMedium   RuleSeverity = "medium"
	SeverityHigh     RuleSeverity = "high"
	SeverityCritical RuleSeverity = "critical"
)

// RuleAction defines what happens when a rule fires
type RuleAction string

const (
	ActionAllow     RuleAction = "allow"
	ActionBlock     RuleAction = "block"
	ActionReview    RuleAction = "review"
	ActionChallenge RuleAction = "challenge"
)

// Rule represents a fraud detection rule
// Rules are configurable and versioned - not hardcoded
type Rule struct {
	ID          uuid.UUID                  `json:"id"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Type        RuleType                   `json:"type"`
	Severity    RuleSeverity               `json:"severity"`
	Action      RuleAction                 `json:"action"`

	// Configuration - JSON blob for flexibility
	// Example for velocity: {"max_transactions": 5, "window_minutes": 5, "amount_threshold": "1000"}
	Config      map[string]interface{}     `json:"config"`

	// Rule management
	Enabled     bool                       `json:"enabled"`
	Version     int                        `json:"version"`
	CreatedBy   uuid.UUID                  `json:"created_by"`

	// Timestamps
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
	EffectiveAt time.Time                  `json:"effective_at"`
	ExpiresAt   *time.Time                 `json:"expires_at,omitempty"`
}

// RuleResult represents the outcome of evaluating a rule
type RuleResult struct {
	RuleID      uuid.UUID                  `json:"rule_id"`
	RuleName    string                     `json:"rule_name"`
	Fired       bool                       `json:"fired"`
	Score       decimal.Decimal            `json:"score"` // 0.0 to 1.0
	Reason      string                     `json:"reason"`
	Action      RuleAction                 `json:"action"`
	Metadata    map[string]interface{}     `json:"metadata,omitempty"`
	EvaluatedAt time.Time                  `json:"evaluated_at"`
}

// RuleEngine evaluates fraud rules against transactions
type RuleEngine interface {
	// Evaluate runs all enabled rules against a transaction context
	Evaluate(ctx context.Context, evalCtx *RuleEvaluationContext) ([]RuleResult, error)

	// EvaluateRule runs a specific rule
	EvaluateRule(ctx context.Context, rule *Rule, evalCtx *RuleEvaluationContext) (*RuleResult, error)

	// GetActiveRules retrieves all currently active rules
	GetActiveRules(ctx context.Context) ([]*Rule, error)

	// AddRule adds a new rule to the engine
	AddRule(ctx context.Context, rule *Rule) error

	// UpdateRule updates an existing rule
	UpdateRule(ctx context.Context, rule *Rule) error

	// DisableRule disables a rule
	DisableRule(ctx context.Context, ruleID uuid.UUID) error
}

// RuleEvaluationContext contains all data needed to evaluate rules
type RuleEvaluationContext struct {
	TransactionID uuid.UUID
	UserID        uuid.UUID
	AccountID     uuid.UUID
	Amount        decimal.Decimal
	Currency      string
	Timestamp     time.Time

	// Context data for different rule types
	Location  *GeoLocation
	Device    *DeviceInfo
	Merchant  *MerchantInfo
	Payment   *PaymentMethod

	// Historical data (fetched from Redis/DB)
	RecentTransactions []TransactionSummary
	UserProfile        *UserProfile
	DeviceHistory      []DeviceRecord
}

// TransactionSummary is a lightweight transaction record for rule evaluation
type TransactionSummary struct {
	ID        uuid.UUID
	Amount    decimal.Decimal
	Timestamp time.Time
	Location  *GeoLocation
	Status    string
}

// UserProfile contains user behavioral patterns
type UserProfile struct {
	UserID             uuid.UUID
	AccountAge         time.Duration
	TypicalLocations   []string
	TypicalMerchants   []string
	AverageTransaction decimal.Decimal
	TrustedDevices     []string
	LastActivityAt     time.Time
}

// DeviceRecord tracks device usage
type DeviceRecord struct {
	DeviceID  string
	FirstSeen time.Time
	LastSeen  time.Time
	TxCount   int
	IsTrusted bool
}

// GeoLocation represents geographic location data
type GeoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Region    string  `json:"region"`
	IPAddress string  `json:"ip_address"`
}

// DeviceInfo captures device fingerprint data
type DeviceInfo struct {
	DeviceID        string    `json:"device_id"`
	DeviceType      string    `json:"device_type"` // mobile, desktop, tablet
	OS              string    `json:"os"`          // iOS, Android, Windows, etc
	Browser         string    `json:"browser"`     // Chrome, Safari, etc
	UserAgent       string    `json:"user_agent"`
	IsTrustedDevice bool      `json:"is_trusted_device"`
	LastSeenAt      time.Time `json:"last_seen_at"`
}

// MerchantInfo contains merchant details
type MerchantInfo struct {
	MerchantID       string `json:"merchant_id"`
	MerchantName     string `json:"merchant_name"`
	MerchantCategory string `json:"merchant_category"` // MCC code
	Country          string `json:"country"`
	IsHighRisk       bool   `json:"is_high_risk"`
}

// PaymentMethod represents payment details
type PaymentMethod struct {
	Type           string `json:"type"`            // card, bank_account, wallet
	Last4          string `json:"last4"`           // Last 4 digits
	Network        string `json:"network"`         // Visa, Mastercard, ACH, etc
	BankID         string `json:"bank_id"`         // Which bank
	IssuingCountry string `json:"issuing_country"`
}

// VelocityRuleConfig defines configuration for velocity checks
type VelocityRuleConfig struct {
	MaxTransactions int             `json:"max_transactions"`
	WindowMinutes   int             `json:"window_minutes"`
	AmountThreshold decimal.Decimal `json:"amount_threshold,omitempty"`
	CountOnly       bool            `json:"count_only"` // Count transactions or sum amounts
}

// AmountRuleConfig defines configuration for amount-based rules
type AmountRuleConfig struct {
	MinAmount       decimal.Decimal `json:"min_amount,omitempty"`
	MaxAmount       decimal.Decimal `json:"max_amount,omitempty"`
	DeviationFactor float64         `json:"deviation_factor,omitempty"` // X times user's average
}

// GeographicRuleConfig defines configuration for location-based rules
type GeographicRuleConfig struct {
	AllowedCountries  []string `json:"allowed_countries,omitempty"`
	BlockedCountries  []string `json:"blocked_countries,omitempty"`
	MaxDistanceKm     float64  `json:"max_distance_km,omitempty"` // Max distance from last known location
	RequireConsistent bool     `json:"require_consistent"`        // Location must match previous pattern
}

// DeviceRuleConfig defines configuration for device-based rules
type DeviceRuleConfig struct {
	RequireTrustedDevice bool `json:"require_trusted_device"`
	MaxDevicesPerUser    int  `json:"max_devices_per_user,omitempty"`
	BlockNewDevices      bool `json:"block_new_devices"`
}

// NewRule creates a new fraud detection rule
func NewRule(name, description string, ruleType RuleType, severity RuleSeverity, action RuleAction, createdBy uuid.UUID) *Rule {
	now := time.Now()
	return &Rule{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Type:        ruleType,
		Severity:    severity,
		Action:      action,
		Config:      make(map[string]interface{}),
		Enabled:     true,
		Version:     1,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		EffectiveAt: now,
	}
}

// IsActive checks if the rule is currently active
func (r *Rule) IsActive() bool {
	now := time.Now()
	if !r.Enabled {
		return false
	}
	if now.Before(r.EffectiveAt) {
		return false
	}
	if r.ExpiresAt != nil && now.After(*r.ExpiresAt) {
		return false
	}
	return true
}

// Disable deactivates the rule
func (r *Rule) Disable() {
	r.Enabled = false
	r.UpdatedAt = time.Now()
}

// Enable activates the rule
func (r *Rule) Enable() {
	r.Enabled = true
	r.UpdatedAt = time.Now()
}

// IncrementVersion creates a new version of the rule
func (r *Rule) IncrementVersion() {
	r.Version++
	r.UpdatedAt = time.Now()
}

// SetExpiration sets when the rule expires
func (r *Rule) SetExpiration(expiresAt time.Time) {
	r.ExpiresAt = &expiresAt
	r.UpdatedAt = time.Now()
}

// UpdateConfig updates the rule configuration
func (r *Rule) UpdateConfig(config map[string]interface{}) {
	r.Config = config
	r.UpdatedAt = time.Now()
}

// NewRuleResult creates a new rule evaluation result
func NewRuleResult(ruleID uuid.UUID, ruleName string, fired bool, score decimal.Decimal, reason string, action RuleAction) *RuleResult {
	return &RuleResult{
		RuleID:      ruleID,
		RuleName:    ruleName,
		Fired:       fired,
		Score:       score,
		Reason:      reason,
		Action:      action,
		Metadata:    make(map[string]interface{}),
		EvaluatedAt: time.Now(),
	}
}

// AddMetadata adds metadata to the rule result
func (rr *RuleResult) AddMetadata(key string, value interface{}) {
	if rr.Metadata == nil {
		rr.Metadata = make(map[string]interface{})
	}
	rr.Metadata[key] = value
}
