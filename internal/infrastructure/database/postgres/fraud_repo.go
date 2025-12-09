package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"fraud-detecction-system/internal/domain/fraud"
)

// FraudDecisionModel is the database model for fraud decisions
type FraudDecisionModel struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey"`
	TransactionID uuid.UUID       `gorm:"type:uuid;index;not null"`
	UserID        uuid.UUID       `gorm:"type:uuid;index;not null"`
	Decision      string          `gorm:"type:varchar(20);not null"`
	Score         decimal.Decimal `gorm:"type:decimal(5,4);not null"`
	RiskLevel     string          `gorm:"type:varchar(20);not null"`
	Confidence    decimal.Decimal `gorm:"type:decimal(5,4)"`
	RulesFired    string          `gorm:"type:jsonb"`
	Reasons       string          `gorm:"type:jsonb"`
	ModelVersion  string          `gorm:"type:varchar(50)"`
	ProcessedAt   time.Time       `gorm:"not null"`
	LatencyMs     int64           `gorm:"not null"`
	CreatedAt     time.Time       `gorm:"not null"`
	UpdatedAt     time.Time       `gorm:"not null"`
}

// TableName returns the table name for fraud decisions
func (FraudDecisionModel) TableName() string {
	return "fraud_decisions"
}

// FraudCaseModel is the database model for fraud cases
type FraudCaseModel struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey"`
	TransactionIDs string           `gorm:"type:jsonb;not null"`
	UserID         uuid.UUID        `gorm:"type:uuid;index;not null"`
	AccountID      uuid.UUID        `gorm:"type:uuid;index;not null"`
	Status         string           `gorm:"type:varchar(20);index;not null"`
	RiskLevel      string           `gorm:"type:varchar(20);not null"`
	TotalAmount    decimal.Decimal  `gorm:"type:decimal(15,2)"`
	Currency       string           `gorm:"type:varchar(3)"`
	AssignedTo     *uuid.UUID       `gorm:"type:uuid;index"`
	Description    string           `gorm:"type:text"`
	Notes          string           `gorm:"type:jsonb"`
	Evidence       string           `gorm:"type:jsonb"`
	Resolution     string           `gorm:"type:text"`
	ResolvedBy     *uuid.UUID       `gorm:"type:uuid"`
	ResolvedAt     *time.Time
	CreatedAt      time.Time        `gorm:"not null"`
	UpdatedAt      time.Time        `gorm:"not null"`
}

// TableName returns the table name for fraud cases
func (FraudCaseModel) TableName() string {
	return "fraud_cases"
}

// RuleModel is the database model for fraud rules
type RuleModel struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name        string     `gorm:"type:varchar(100);uniqueIndex;not null"`
	Description string     `gorm:"type:text"`
	Type        string     `gorm:"type:varchar(20);index;not null"`
	Severity    string     `gorm:"type:varchar(20);not null"`
	Action      string     `gorm:"type:varchar(20);not null"`
	Config      string     `gorm:"type:jsonb;not null"`
	Enabled     bool       `gorm:"index;not null"`
	Version     int        `gorm:"not null"`
	CreatedBy   uuid.UUID  `gorm:"type:uuid;not null"`
	CreatedAt   time.Time  `gorm:"not null"`
	UpdatedAt   time.Time  `gorm:"not null"`
	EffectiveAt time.Time  `gorm:"not null"`
	ExpiresAt   *time.Time
}

// TableName returns the table name for fraud rules
func (RuleModel) TableName() string {
	return "fraud_rules"
}

// DecisionRepository implements fraud.DecisionRepository
type DecisionRepository struct {
	db *gorm.DB
}

// NewDecisionRepository creates a new decision repository
func NewDecisionRepository(client *Client) *DecisionRepository {
	return &DecisionRepository{db: client.DB()}
}

// Create stores a fraud decision
func (r *DecisionRepository) Create(ctx context.Context, decision *fraud.FraudDecision) error {
	rulesFired, _ := json.Marshal(decision.RulesFired)
	reasons, _ := json.Marshal(decision.Reasons)

	model := &FraudDecisionModel{
		ID:            decision.ID,
		TransactionID: decision.TransactionID,
		UserID:        decision.UserID,
		Decision:      string(decision.Decision),
		Score:         decision.Score,
		RiskLevel:     string(decision.RiskLevel),
		Confidence:    decision.Confidence,
		RulesFired:    string(rulesFired),
		Reasons:       string(reasons),
		ModelVersion:  decision.ModelVersion,
		ProcessedAt:   decision.ProcessedAt,
		LatencyMs:     decision.LatencyMs,
		CreatedAt:     decision.CreatedAt,
		UpdatedAt:     decision.UpdatedAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

// GetByID retrieves a decision by ID
func (r *DecisionRepository) GetByID(ctx context.Context, id uuid.UUID) (*fraud.FraudDecision, error) {
	var model FraudDecisionModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fraud.ErrDecisionNotFound
		}
		return nil, err
	}
	return modelToDecision(&model), nil
}

// GetByTransactionID retrieves decision for a transaction
func (r *DecisionRepository) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*fraud.FraudDecision, error) {
	var model FraudDecisionModel
	if err := r.db.WithContext(ctx).First(&model, "transaction_id = ?", transactionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fraud.ErrDecisionNotFound
		}
		return nil, err
	}
	return modelToDecision(&model), nil
}

// ListByUserID gets fraud decisions for a user
func (r *DecisionRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*fraud.FraudDecision, error) {
	var models []FraudDecisionModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error; err != nil {
		return nil, err
	}

	decisions := make([]*fraud.FraudDecision, len(models))
	for i, m := range models {
		decisions[i] = modelToDecision(&m)
	}
	return decisions, nil
}

// GetBlockedCount counts how many times a user has been blocked
func (r *DecisionRepository) GetBlockedCount(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&FraudDecisionModel{}).
		Where("user_id = ? AND decision = ? AND created_at >= ?", userID, "block", since).
		Count(&count).Error
	return count, err
}

func modelToDecision(m *FraudDecisionModel) *fraud.FraudDecision {
	var rulesFired []string
	var reasons []string
	json.Unmarshal([]byte(m.RulesFired), &rulesFired)
	json.Unmarshal([]byte(m.Reasons), &reasons)

	return &fraud.FraudDecision{
		ID:            m.ID,
		TransactionID: m.TransactionID,
		UserID:        m.UserID,
		Decision:      fraud.DecisionType(m.Decision),
		Score:         m.Score,
		RiskLevel:     fraud.RiskLevel(m.RiskLevel),
		Confidence:    m.Confidence,
		RulesFired:    rulesFired,
		Reasons:       reasons,
		ModelVersion:  m.ModelVersion,
		ProcessedAt:   m.ProcessedAt,
		LatencyMs:     m.LatencyMs,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

// CaseRepository implements fraud.CaseRepository
type CaseRepository struct {
	db *gorm.DB
}

// NewCaseRepository creates a new case repository
func NewCaseRepository(client *Client) *CaseRepository {
	return &CaseRepository{db: client.DB()}
}

// Create stores a new fraud case
func (r *CaseRepository) Create(ctx context.Context, fraudCase *fraud.FraudCase) error {
	transactionIDs, _ := json.Marshal(fraudCase.TransactionIDs)
	notes, _ := json.Marshal(fraudCase.Notes)
	evidence, _ := json.Marshal(fraudCase.Evidence)

	model := &FraudCaseModel{
		ID:             fraudCase.ID,
		TransactionIDs: string(transactionIDs),
		UserID:         fraudCase.UserID,
		AccountID:      fraudCase.AccountID,
		Status:         string(fraudCase.Status),
		RiskLevel:      string(fraudCase.RiskLevel),
		TotalAmount:    fraudCase.TotalAmount,
		Currency:       fraudCase.Currency,
		AssignedTo:     fraudCase.AssignedTo,
		Description:    fraudCase.Description,
		Notes:          string(notes),
		Evidence:       string(evidence),
		Resolution:     fraudCase.Resolution,
		ResolvedBy:     fraudCase.ResolvedBy,
		ResolvedAt:     fraudCase.ResolvedAt,
		CreatedAt:      fraudCase.CreatedAt,
		UpdatedAt:      fraudCase.UpdatedAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

// GetByID retrieves a case by ID
func (r *CaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*fraud.FraudCase, error) {
	var model FraudCaseModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fraud.ErrCaseNotFound
		}
		return nil, err
	}
	return modelToCase(&model), nil
}

// Update updates an existing case
func (r *CaseRepository) Update(ctx context.Context, fraudCase *fraud.FraudCase) error {
	transactionIDs, _ := json.Marshal(fraudCase.TransactionIDs)
	notes, _ := json.Marshal(fraudCase.Notes)
	evidence, _ := json.Marshal(fraudCase.Evidence)

	return r.db.WithContext(ctx).Model(&FraudCaseModel{}).
		Where("id = ?", fraudCase.ID).
		Updates(map[string]interface{}{
			"transaction_ids": string(transactionIDs),
			"status":          string(fraudCase.Status),
			"risk_level":      string(fraudCase.RiskLevel),
			"total_amount":    fraudCase.TotalAmount,
			"currency":        fraudCase.Currency,
			"assigned_to":     fraudCase.AssignedTo,
			"description":     fraudCase.Description,
			"notes":           string(notes),
			"evidence":        string(evidence),
			"resolution":      fraudCase.Resolution,
			"resolved_by":     fraudCase.ResolvedBy,
			"resolved_at":     fraudCase.ResolvedAt,
			"updated_at":      time.Now(),
		}).Error
}

// ListByStatus retrieves cases by status
func (r *CaseRepository) ListByStatus(ctx context.Context, status fraud.CaseStatus, limit, offset int) ([]*fraud.FraudCase, error) {
	var models []FraudCaseModel
	if err := r.db.WithContext(ctx).
		Where("status = ?", string(status)).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error; err != nil {
		return nil, err
	}

	cases := make([]*fraud.FraudCase, len(models))
	for i, m := range models {
		cases[i] = modelToCase(&m)
	}
	return cases, nil
}

// ListByAssignee retrieves cases assigned to an investigator
func (r *CaseRepository) ListByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*fraud.FraudCase, error) {
	var models []FraudCaseModel
	if err := r.db.WithContext(ctx).
		Where("assigned_to = ?", assigneeID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error; err != nil {
		return nil, err
	}

	cases := make([]*fraud.FraudCase, len(models))
	for i, m := range models {
		cases[i] = modelToCase(&m)
	}
	return cases, nil
}

// GetOpenCasesByUser checks if user has any open fraud cases
func (r *CaseRepository) GetOpenCasesByUser(ctx context.Context, userID uuid.UUID) ([]*fraud.FraudCase, error) {
	var models []FraudCaseModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status IN ?", userID, []string{"open", "investigating"}).
		Find(&models).Error; err != nil {
		return nil, err
	}

	cases := make([]*fraud.FraudCase, len(models))
	for i, m := range models {
		cases[i] = modelToCase(&m)
	}
	return cases, nil
}

func modelToCase(m *FraudCaseModel) *fraud.FraudCase {
	var transactionIDs []uuid.UUID
	var notes []fraud.CaseNote
	var evidence []fraud.Evidence
	json.Unmarshal([]byte(m.TransactionIDs), &transactionIDs)
	json.Unmarshal([]byte(m.Notes), &notes)
	json.Unmarshal([]byte(m.Evidence), &evidence)

	return &fraud.FraudCase{
		ID:             m.ID,
		TransactionIDs: transactionIDs,
		UserID:         m.UserID,
		AccountID:      m.AccountID,
		Status:         fraud.CaseStatus(m.Status),
		RiskLevel:      fraud.RiskLevel(m.RiskLevel),
		TotalAmount:    m.TotalAmount,
		Currency:       m.Currency,
		AssignedTo:     m.AssignedTo,
		Description:    m.Description,
		Notes:          notes,
		Evidence:       evidence,
		Resolution:     m.Resolution,
		ResolvedBy:     m.ResolvedBy,
		ResolvedAt:     m.ResolvedAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

// RuleRepository implements fraud.RuleRepository
type RuleRepository struct {
	db *gorm.DB
}

// NewRuleRepository creates a new rule repository
func NewRuleRepository(client *Client) *RuleRepository {
	return &RuleRepository{db: client.DB()}
}

// Create adds a new rule
func (r *RuleRepository) Create(ctx context.Context, rule *fraud.Rule) error {
	config, _ := json.Marshal(rule.Config)

	model := &RuleModel{
		ID:          rule.ID,
		Name:        rule.Name,
		Description: rule.Description,
		Type:        string(rule.Type),
		Severity:    string(rule.Severity),
		Action:      string(rule.Action),
		Config:      string(config),
		Enabled:     rule.Enabled,
		Version:     rule.Version,
		CreatedBy:   rule.CreatedBy,
		CreatedAt:   rule.CreatedAt,
		UpdatedAt:   rule.UpdatedAt,
		EffectiveAt: rule.EffectiveAt,
		ExpiresAt:   rule.ExpiresAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

// GetByID retrieves a rule by ID
func (r *RuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*fraud.Rule, error) {
	var model RuleModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fraud.ErrRuleNotFound
		}
		return nil, err
	}
	return modelToRule(&model), nil
}

// Update updates an existing rule
func (r *RuleRepository) Update(ctx context.Context, rule *fraud.Rule) error {
	config, _ := json.Marshal(rule.Config)

	return r.db.WithContext(ctx).Model(&RuleModel{}).
		Where("id = ?", rule.ID).
		Updates(map[string]interface{}{
			"name":         rule.Name,
			"description":  rule.Description,
			"type":         string(rule.Type),
			"severity":     string(rule.Severity),
			"action":       string(rule.Action),
			"config":       string(config),
			"enabled":      rule.Enabled,
			"version":      rule.Version,
			"updated_at":   time.Now(),
			"effective_at": rule.EffectiveAt,
			"expires_at":   rule.ExpiresAt,
		}).Error
}

// ListActive retrieves all enabled rules
func (r *RuleRepository) ListActive(ctx context.Context) ([]*fraud.Rule, error) {
	var models []RuleModel
	now := time.Now().UTC() // Use UTC to match database timestamps
	if err := r.db.WithContext(ctx).
		Where("enabled = ? AND effective_at <= ? AND (expires_at IS NULL OR expires_at > ?)", true, now, now).
		Find(&models).Error; err != nil {
		return nil, err
	}

	rules := make([]*fraud.Rule, len(models))
	for i, m := range models {
		rules[i] = modelToRule(&m)
	}
	return rules, nil
}

// ListByType retrieves rules of a specific type
func (r *RuleRepository) ListByType(ctx context.Context, ruleType fraud.RuleType) ([]*fraud.Rule, error) {
	var models []RuleModel
	if err := r.db.WithContext(ctx).
		Where("type = ? AND enabled = ?", string(ruleType), true).
		Find(&models).Error; err != nil {
		return nil, err
	}

	rules := make([]*fraud.Rule, len(models))
	for i, m := range models {
		rules[i] = modelToRule(&m)
	}
	return rules, nil
}

// Disable disables a rule
func (r *RuleRepository) Disable(ctx context.Context, ruleID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&RuleModel{}).
		Where("id = ?", ruleID).
		Updates(map[string]interface{}{
			"enabled":    false,
			"updated_at": time.Now(),
		}).Error
}

// GetVersion retrieves a specific version of a rule
func (r *RuleRepository) GetVersion(ctx context.Context, ruleID uuid.UUID, version int) (*fraud.Rule, error) {
	var model RuleModel
	if err := r.db.WithContext(ctx).First(&model, "id = ? AND version = ?", ruleID, version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fraud.ErrRuleNotFound
		}
		return nil, err
	}
	return modelToRule(&model), nil
}

func modelToRule(m *RuleModel) *fraud.Rule {
	var config map[string]interface{}
	json.Unmarshal([]byte(m.Config), &config)

	return &fraud.Rule{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Type:        fraud.RuleType(m.Type),
		Severity:    fraud.RuleSeverity(m.Severity),
		Action:      fraud.RuleAction(m.Action),
		Config:      config,
		Enabled:     m.Enabled,
		Version:     m.Version,
		CreatedBy:   m.CreatedBy,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		EffectiveAt: m.EffectiveAt,
		ExpiresAt:   m.ExpiresAt,
	}
}

