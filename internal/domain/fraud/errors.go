package fraud

import "errors"

var (
	// Decision errors
	ErrDecisionNotFound    = errors.New("fraud decision not found")
	ErrInvalidScore        = errors.New("invalid fraud score: must be between 0 and 1")
	ErrInvalidRiskLevel    = errors.New("invalid risk level")
	ErrInvalidDecisionType = errors.New("invalid decision type")

	// Case errors
	ErrCaseNotFound      = errors.New("fraud case not found")
	ErrCaseAlreadyClosed = errors.New("case is already closed")
	ErrCaseNotResolved   = errors.New("case must be resolved before closing")
	ErrInvalidCaseStatus = errors.New("invalid case status")
	ErrCaseNotAssigned   = errors.New("case is not assigned to an investigator")

	// Rule errors
	ErrRuleNotFound         = errors.New("fraud rule not found")
	ErrRuleAlreadyExists    = errors.New("rule with this name already exists")
	ErrInvalidRuleType      = errors.New("invalid rule type")
	ErrInvalidRuleSeverity  = errors.New("invalid rule severity")
	ErrInvalidRuleAction    = errors.New("invalid rule action")
	ErrRuleConfigInvalid    = errors.New("rule configuration is invalid")
	ErrRuleNotActive        = errors.New("rule is not active")
	ErrRuleVersionMismatch  = errors.New("rule version mismatch")

	// Evaluation errors
	ErrEvaluationFailed       = errors.New("rule evaluation failed")
	ErrInsufficientData       = errors.New("insufficient data for fraud evaluation")
	ErrMissingTransactionData = errors.New("missing required transaction data")
	ErrMissingUserProfile     = errors.New("missing user profile data")
	ErrScoringFailed          = errors.New("fraud scoring calculation failed")

	// Analysis errors
	ErrAnalysisTimeout = errors.New("fraud analysis timed out")
	ErrModelUnavailable = errors.New("ML model is unavailable")
)
