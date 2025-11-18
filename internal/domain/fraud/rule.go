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
}