package fraud

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// RuleType categories the type of fraud rule type RuleType string

const (
	RuleTypeVelocity RuleType = "velocity"
	RuleTypeAmount  RuleType = "amount"
	RuleTypeGeographic RuleType = "geographic"
	RuleTypeDevice RuleType = "device"
	RuleTypeMerchance  RuleType = "merchandise"
	RuleTypeBehavior RuleType = "behaviorial"
)

// RuleSeverity indicates how serious a rule violation is RuleSeverity string

const (
	SeverityLow	RuleSeverity = "low"
	SeverityMedium RuleSeverity = "medium"
	SeverityHigh RuleSeverity = "high"
	SeverityCritical RuleSeverity = "critical"
)

// RuleAction defines what happens when a rule fires type RuleAction string

const (
	ActionAllow RuleAction = "allow"
	ActionBlock RuleAction = "block"
	ActionReview RuleAction = "review"
	ActionChallenge RuleAction = "challenge"
)

// Rule represents a fraud detection rule
// Rules are configurable and versioned - not hardcoded
type Reule struct {
	ID		  uuid.UUID     `json:"id"`
	Name 	  string        `json:"name"`
	Description string        `json:"description"`
	Type 	  RuleType      `json:"type"`
	Severity   RuleSeverity  `json:"severity"`
	Action 	  RuleAction    `json:"action"`

	// Configuration - JSON blob for flexibility
	// Example for velocity: { "max_transactions": 5, "time_window_minutes": 10, "ammount_threshold": "1000" }
	Config 	  map[string]interface{} `json:"config"`

	// Rule management
	Enabled bool		  `json:"enabled"`
	Version int		  `json:"version"`
	CreatedBy uuid.UUID     `json:"created_by"`

	// Timestamps
	CreatedAt time.time   `json:"created_at"`
	UpdatedAt time.time   `json:"updated_at"`
	EffectedAt time.time   `json:"effected_at"`
	ExpiredAt  *time.time  `json:"expired_at,omitempty"`
}

// RuleEvaluationContext contains all data needed to evaluate rules
type RuleEvaluationContext struct {
	TransactionID uuid.UUID           `json:"transaction_id"`
	UserId uuid.UUID			  `json:"user_id"`	
	AccountID uuid.UUID		  `json:"account_id"`
	Amount decimal.Decimal	  `json:"amount"`
	Currency string 		  `json:"currency"`
	Timestamp time.Time		  `json:"timestamp"`

	// Context data for different rule types
	Location *GeoLocation
	Device *DeviceInfo
	Merchant *MerchantInfo
	Paymen *PaymentInfo

	// Historical data ( fetched from Redis/DB )
	RecentTrabsactions []TransactionSummary
	UserProfile *UserProfile
	DeviceHistory []DeviceRecord
}

// TransactionSummary is a lightweight transaction representation for rule evaluation
type TransactionSummary struct {
	ID uuid.UUID		  `json:"id"`
	Amount decimal.Decimal	  `json:"amount"`
	Timestamp time.Time		  `json:"timestamp"`
	Location *GeoLocation	  `json:"location,omitempty"`
	Status string		  `json:"status"`
}
