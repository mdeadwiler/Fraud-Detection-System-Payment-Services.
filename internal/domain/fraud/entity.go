package fraud

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// RiskLevel represents the severity of fraud risk
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// CaseStatus represents the state of a fraud case
type CaseStatus string

const (
	CaseStatusOpen       CaseStatus = "open"
	CaseStatusInvestigating CaseStatus = "investigating"
	CaseStatusResolved   CaseStatus = "resolved"
	CaseStatusClosed     CaseStatus = "closed"
	CaseStatusEscalated  CaseStatus = "escalated"
)

// DecisionType represents the fraud detection decision
type DecisionType string

const (
	DecisionAllow   DecisionType = "allow"
	DecisionBlock   DecisionType = "block"
	DecisionReview  DecisionType = "review"
	DecisionChallenge DecisionType = "challenge" // Requires additional verification
)

// FraudDecision represents the outcome of fraud analysis on a transaction
// This is what the fraud detection engine produces after analyzing a transaction
type FraudDecision struct {
	ID            uuid.UUID        `json:"id"`
	TransactionID uuid.UUID        `json:"transaction_id"`
	UserID        uuid.UUID        `json:"user_id"`

	// Decision Details
	Decision      DecisionType     `json:"decision"`
	Score         decimal.Decimal  `json:"score"`          // 0.0 to 1.0 fraud probability
	RiskLevel     RiskLevel        `json:"risk_level"`
	Confidence    decimal.Decimal  `json:"confidence"`     // Model confidence 0.0 to 1.0

	// Explanation
	RulesFired    []string         `json:"rules_fired"`    // Which rules triggered
	Reasons       []string         `json:"reasons"`        // Human-readable explanations
	ModelVersion  string           `json:"model_version"`  // Which ML model version was used

	// Metadata
	ProcessedAt   time.Time        `json:"processed_at"`
	LatencyMs     int64            `json:"latency_ms"`     // How long fraud check took

	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// NewFraudDecision creates a new fraud decision
func NewFraudDecision(transactionID, userID uuid.UUID, decision DecisionType, score decimal.Decimal) *FraudDecision {
	now := time.Now()
	return &FraudDecision{
		ID:            uuid.New(),
		TransactionID: transactionID,
		UserID:        userID,
		Decision:      decision,
		Score:         score,
		RulesFired:    make([]string, 0),
		Reasons:       make([]string, 0),
		ProcessedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// AddRule adds a fired rule to the decision
func (fd *FraudDecision) AddRule(ruleName string) {
	fd.RulesFired = append(fd.RulesFired, ruleName)
	fd.UpdatedAt = time.Now()
}

// AddReason adds a human-readable reason for the decision
func (fd *FraudDecision) AddReason(reason string) {
	fd.Reasons = append(fd.Reasons, reason)
	fd.UpdatedAt = time.Now()
}

// SetRiskLevel sets the risk level based on score
func (fd *FraudDecision) SetRiskLevel() {
	score := fd.Score.InexactFloat64()
	switch {
	case score >= 0.80:
		fd.RiskLevel = RiskLevelCritical
	case score >= 0.60:
		fd.RiskLevel = RiskLevelHigh
	case score >= 0.30:
		fd.RiskLevel = RiskLevelMedium
	default:
		fd.RiskLevel = RiskLevelLow
	}
	fd.UpdatedAt = time.Now()
}

// ShouldBlock determines if this decision should block the transaction
func (fd *FraudDecision) ShouldBlock() bool {
	return fd.Decision == DecisionBlock
}

// RequiresReview determines if this decision requires manual review
func (fd *FraudDecision) RequiresReview() bool {
	return fd.Decision == DecisionReview
}

// FraudCase represents an investigated fraud incident
// This is created when transactions are flagged for manual review
type FraudCase struct {
	ID              uuid.UUID         `json:"id"`
	TransactionIDs  []uuid.UUID       `json:"transaction_ids"`  // Multiple related transactions
	UserID          uuid.UUID         `json:"user_id"`
	AccountID       uuid.UUID         `json:"account_id"`

	// Case Details
	Status          CaseStatus        `json:"status"`
	RiskLevel       RiskLevel         `json:"risk_level"`
	TotalAmount     decimal.Decimal   `json:"total_amount"`
	Currency        string            `json:"currency"`

	// Investigation
	AssignedTo      *uuid.UUID        `json:"assigned_to,omitempty"`
	Description     string            `json:"description"`
	Notes           []CaseNote        `json:"notes"`
	Evidence        []Evidence        `json:"evidence"`

	// Resolution
	Resolution      string            `json:"resolution,omitempty"`
	ResolvedBy      *uuid.UUID        `json:"resolved_by,omitempty"`
	ResolvedAt      *time.Time        `json:"resolved_at,omitempty"`

	// Timestamps
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// CaseNote represents a note added to a fraud case during investigation
type CaseNote struct {
	ID        uuid.UUID  `json:"id"`
	Author    uuid.UUID  `json:"author"`
	Content   string     `json:"content"`
	CreatedAt time.Time  `json:"created_at"`
}

// Evidence represents evidence collected for a fraud case
type Evidence struct {
	ID          uuid.UUID         `json:"id"`
	Type        string            `json:"type"`         // screenshot, log, document
	Description string            `json:"description"`
	URL         string            `json:"url,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// NewFraudCase creates a new fraud case
func NewFraudCase(transactionID, userID, accountID uuid.UUID, riskLevel RiskLevel) *FraudCase {
	now := time.Now()
	return &FraudCase{
		ID:             uuid.New(),
		TransactionIDs: []uuid.UUID{transactionID},
		UserID:         userID,
		AccountID:      accountID,
		Status:         CaseStatusOpen,
		RiskLevel:      riskLevel,
		Notes:          make([]CaseNote, 0),
		Evidence:       make([]Evidence, 0),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Assign assigns the case to an investigator
func (fc *FraudCase) Assign(investigatorID uuid.UUID) error {
	if fc.Status == CaseStatusClosed || fc.Status == CaseStatusResolved {
		return ErrCaseAlreadyClosed
	}
	fc.AssignedTo = &investigatorID
	fc.Status = CaseStatusInvestigating
	fc.UpdatedAt = time.Now()
	return nil
}

// AddNote adds a note to the case
func (fc *FraudCase) AddNote(authorID uuid.UUID, content string) {
	note := CaseNote{
		ID:        uuid.New(),
		Author:    authorID,
		Content:   content,
		CreatedAt: time.Now(),
	}
	fc.Notes = append(fc.Notes, note)
	fc.UpdatedAt = time.Now()
}

// AddEvidence adds evidence to the case
func (fc *FraudCase) AddEvidence(evidenceType, description, url string, metadata map[string]string) {
	evidence := Evidence{
		ID:          uuid.New(),
		Type:        evidenceType,
		Description: description,
		URL:         url,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}
	fc.Evidence = append(fc.Evidence, evidence)
	fc.UpdatedAt = time.Now()
}

// Resolve marks the case as resolved
func (fc *FraudCase) Resolve(resolverID uuid.UUID, resolution string) error {
	if fc.Status == CaseStatusClosed {
		return ErrCaseAlreadyClosed
	}
	fc.Status = CaseStatusResolved
	fc.Resolution = resolution
	fc.ResolvedBy = &resolverID
	now := time.Now()
	fc.ResolvedAt = &now
	fc.UpdatedAt = now
	return nil
}

// Close closes the case
func (fc *FraudCase) Close() error {
	if fc.Status != CaseStatusResolved {
		return ErrCaseNotResolved
	}
	fc.Status = CaseStatusClosed
	fc.UpdatedAt = time.Now()
	return nil
}

// Escalate escalates the case to a higher authority
func (fc *FraudCase) Escalate(reason string) {
	fc.Status = CaseStatusEscalated
	fc.AddNote(uuid.Nil, "Case escalated: "+reason)
	fc.UpdatedAt = time.Now()
}

// IsOpen checks if the case is still open
func (fc *FraudCase) IsOpen() bool {
	return fc.Status == CaseStatusOpen || fc.Status == CaseStatusInvestigating
}

// IsClosed checks if the case is closed
func (fc *FraudCase) IsClosed() bool {
	return fc.Status == CaseStatusClosed
}

// AddTransaction adds a related transaction to the case
func (fc *FraudCase) AddTransaction(transactionID uuid.UUID) {
	fc.TransactionIDs = append(fc.TransactionIDs, transactionID)
	fc.UpdatedAt = time.Now()
}
