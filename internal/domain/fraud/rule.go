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