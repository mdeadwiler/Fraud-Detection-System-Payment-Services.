package fraud

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Service handles fraud detection business logic
// This is the core domain service that orchestrates fraud analysis
type Service struct {
	decisionRepo DecisionRepository
	caseRepo     CaseRepository
	ruleRepo     RuleRepository
	ruleEngine   RuleEngine
	scorer       FraudScorer

	// Configuration
	decisionThresholds DecisionThresholds
	scoreWeights       ScoreWeights
	scoringStrategy    ScoringStrategy
}

// NewService creates a new fraud detection service
func NewService(
	decisionRepo DecisionRepository,
	caseRepo CaseRepository,
	ruleRepo RuleRepository,
	ruleEngine RuleEngine,
	scorer FraudScorer,
) *Service {
	return &Service{
		decisionRepo:       decisionRepo,
		caseRepo:           caseRepo,
		ruleRepo:           ruleRepo,
		ruleEngine:         ruleEngine,
		scorer:             scorer,
		decisionThresholds: DefaultDecisionThresholds(),
		scoreWeights:       DefaultScoreWeights(),
		scoringStrategy:    StrategyWeightedAverage,
	}
}

// AnalyzeTransaction performs fraud analysis on a transaction
// This is the main entry point for fraud detection
func (s *Service) AnalyzeTransaction(ctx context.Context, evalCtx *RuleEvaluationContext) (*FraudDecision, error) {
	startTime := time.Now()

	// Validate input
	if evalCtx == nil {
		return nil, ErrMissingTransactionData
	}
	if evalCtx.UserID == uuid.Nil || evalCtx.TransactionID == uuid.Nil {
		return nil, ErrMissingTransactionData
	}

	// Evaluate all active rules
	ruleResults, err := s.ruleEngine.Evaluate(ctx, evalCtx)
	if err != nil {
		return nil, ErrEvaluationFailed
	}

	// Calculate aggregate fraud score
	scoreResult, err := AggregateRuleResults(ruleResults, s.scoreWeights, s.scoringStrategy)
	if err != nil {
		return nil, ErrScoringFailed
	}

	// Determine decision based on score
	decision := s.determineDecision(scoreResult.FinalScore)

	// Create fraud decision
	fraudDecision := NewFraudDecision(
		evalCtx.TransactionID,
		evalCtx.UserID,
		decision,
		scoreResult.FinalScore,
	)

	// Populate decision details
	fraudDecision.RiskLevel = scoreResult.RiskLevel
	fraudDecision.Confidence = s.calculateConfidence(ruleResults)
	fraudDecision.ProcessedAt = time.Now()
	fraudDecision.LatencyMs = time.Since(startTime).Milliseconds()

	// Add fired rules and reasons
	for _, result := range ruleResults {
		if result.Fired {
			fraudDecision.AddRule(result.RuleName)
			fraudDecision.AddReason(result.Reason)
		}
	}

	// Persist decision
	if err := s.decisionRepo.Create(ctx, fraudDecision); err != nil {
		return nil, err
	}

	// If flagged for review, create a fraud case
	if decision == DecisionReview || decision == DecisionBlock {
		if err := s.createFraudCaseIfNeeded(ctx, evalCtx, fraudDecision); err != nil {
			// Log error but don't fail the analysis
			// Creating a case is secondary to making the fraud decision
		}
	}

	return fraudDecision, nil
}

// GetDecision retrieves a fraud decision by ID
func (s *Service) GetDecision(ctx context.Context, decisionID uuid.UUID) (*FraudDecision, error) {
	return s.decisionRepo.GetByID(ctx, decisionID)
}

// GetDecisionByTransaction retrieves a fraud decision for a transaction
func (s *Service) GetDecisionByTransaction(ctx context.Context, transactionID uuid.UUID) (*FraudDecision, error) {
	return s.decisionRepo.GetByTransactionID(ctx, transactionID)
}

// GetUserBlockedCount returns how many times a user has been blocked
func (s *Service) GetUserBlockedCount(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	return s.decisionRepo.GetBlockedCount(ctx, userID, since)
}

// CreateFraudCase creates a new fraud investigation case
func (s *Service) CreateFraudCase(ctx context.Context, transactionID, userID, accountID uuid.UUID, riskLevel RiskLevel, description string) (*FraudCase, error) {
	// Check if user already has open cases
	openCases, err := s.caseRepo.GetOpenCasesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If user has existing open cases, add transaction to existing case
	if len(openCases) > 0 {
		existingCase := openCases[0]
		existingCase.AddTransaction(transactionID)
		if err := s.caseRepo.Update(ctx, existingCase); err != nil {
			return nil, err
		}
		return existingCase, nil
	}

	// Create new case
	fraudCase := NewFraudCase(transactionID, userID, accountID, riskLevel)
	fraudCase.Description = description

	if err := s.caseRepo.Create(ctx, fraudCase); err != nil {
		return nil, err
	}

	return fraudCase, nil
}

// GetCase retrieves a fraud case by ID
func (s *Service) GetCase(ctx context.Context, caseID uuid.UUID) (*FraudCase, error) {
	return s.caseRepo.GetByID(ctx, caseID)
}

// AssignCase assigns a case to an investigator
func (s *Service) AssignCase(ctx context.Context, caseID, investigatorID uuid.UUID) error {
	fraudCase, err := s.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return err
	}

	if err := fraudCase.Assign(investigatorID); err != nil {
		return err
	}

	return s.caseRepo.Update(ctx, fraudCase)
}

// AddCaseNote adds a note to a fraud case
func (s *Service) AddCaseNote(ctx context.Context, caseID, authorID uuid.UUID, content string) error {
	fraudCase, err := s.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return err
	}

	fraudCase.AddNote(authorID, content)
	return s.caseRepo.Update(ctx, fraudCase)
}

// ResolveCase marks a case as resolved
func (s *Service) ResolveCase(ctx context.Context, caseID, resolverID uuid.UUID, resolution string) error {
	fraudCase, err := s.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return err
	}

	if err := fraudCase.Resolve(resolverID, resolution); err != nil {
		return err
	}

	return s.caseRepo.Update(ctx, fraudCase)
}

// CloseCase closes a resolved case
func (s *Service) CloseCase(ctx context.Context, caseID uuid.UUID) error {
	fraudCase, err := s.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return err
	}

	if err := fraudCase.Close(); err != nil {
		return err
	}

	return s.caseRepo.Update(ctx, fraudCase)
}

// EscalateCase escalates a case to higher authority
func (s *Service) EscalateCase(ctx context.Context, caseID uuid.UUID, reason string) error {
	fraudCase, err := s.caseRepo.GetByID(ctx, caseID)
	if err != nil {
		return err
	}

	fraudCase.Escalate(reason)
	return s.caseRepo.Update(ctx, fraudCase)
}

// ListCasesByStatus retrieves cases by status
func (s *Service) ListCasesByStatus(ctx context.Context, status CaseStatus, limit, offset int) ([]*FraudCase, error) {
	return s.caseRepo.ListByStatus(ctx, status, limit, offset)
}

// ListCasesByAssignee retrieves cases assigned to an investigator
func (s *Service) ListCasesByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*FraudCase, error) {
	return s.caseRepo.ListByAssignee(ctx, assigneeID, limit, offset)
}

// CreateRule creates a new fraud detection rule
func (s *Service) CreateRule(ctx context.Context, rule *Rule) error {
	// Validate rule
	if err := s.validateRule(rule); err != nil {
		return err
	}

	return s.ruleRepo.Create(ctx, rule)
}

// UpdateRule updates an existing rule
func (s *Service) UpdateRule(ctx context.Context, rule *Rule) error {
	// Validate rule
	if err := s.validateRule(rule); err != nil {
		return err
	}

	// Increment version for audit trail
	rule.IncrementVersion()

	return s.ruleRepo.Update(ctx, rule)
}

// GetRule retrieves a rule by ID
func (s *Service) GetRule(ctx context.Context, ruleID uuid.UUID) (*Rule, error) {
	return s.ruleRepo.GetByID(ctx, ruleID)
}

// ListActiveRules retrieves all active rules
func (s *Service) ListActiveRules(ctx context.Context) ([]*Rule, error) {
	return s.ruleRepo.ListActive(ctx)
}

// DisableRule disables a rule
func (s *Service) DisableRule(ctx context.Context, ruleID uuid.UUID) error {
	return s.ruleRepo.Disable(ctx, ruleID)
}

// EnableRule enables a disabled rule
func (s *Service) EnableRule(ctx context.Context, ruleID uuid.UUID) error {
	rule, err := s.ruleRepo.GetByID(ctx, ruleID)
	if err != nil {
		return err
	}

	rule.Enable()
	return s.ruleRepo.Update(ctx, rule)
}

// GetUserRiskProfile analyzes a user's risk profile
func (s *Service) GetUserRiskProfile(ctx context.Context, userID uuid.UUID) (*UserRiskProfile, error) {
	// Get recent decisions
	recentDecisions, err := s.decisionRepo.ListByUserID(ctx, userID, 100, 0)
	if err != nil {
		return nil, err
	}

	// Get blocked count in last 30 days
	since := time.Now().AddDate(0, 0, -30)
	blockedCount, err := s.decisionRepo.GetBlockedCount(ctx, userID, since)
	if err != nil {
		return nil, err
	}

	// Get open cases
	openCases, err := s.caseRepo.GetOpenCasesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Calculate risk metrics
	riskProfile := &UserRiskProfile{
		UserID:            userID,
		BlockedCount:      blockedCount,
		OpenCasesCount:    int64(len(openCases)),
		RecentDecisions:   len(recentDecisions),
		AnalyzedAt:        time.Now(),
	}

	// Calculate average risk score from recent decisions
	if len(recentDecisions) > 0 {
		totalScore := decimal.Zero
		for _, decision := range recentDecisions {
			totalScore = totalScore.Add(decision.Score)
		}
		riskProfile.AverageRiskScore = totalScore.Div(decimal.NewFromInt(int64(len(recentDecisions))))
	}

	// Determine overall risk level
	riskProfile.RiskLevel = s.determineUserRiskLevel(riskProfile)

	return riskProfile, nil
}

// UserRiskProfile represents a user's risk assessment
type UserRiskProfile struct {
	UserID           uuid.UUID       `json:"user_id"`
	RiskLevel        RiskLevel       `json:"risk_level"`
	AverageRiskScore decimal.Decimal `json:"average_risk_score"`
	BlockedCount     int64           `json:"blocked_count"`
	OpenCasesCount   int64           `json:"open_cases_count"`
	RecentDecisions  int             `json:"recent_decisions"`
	AnalyzedAt       time.Time       `json:"analyzed_at"`
}

// Private helper methods

func (s *Service) determineDecision(score decimal.Decimal) DecisionType {
	// Check thresholds in order of severity
	if score.GreaterThanOrEqual(s.decisionThresholds.BlockThreshold) {
		return DecisionBlock
	}
	if score.GreaterThanOrEqual(s.decisionThresholds.ReviewThreshold) {
		return DecisionReview
	}
	if score.GreaterThanOrEqual(s.decisionThresholds.ChallengeThreshold) {
		return DecisionChallenge
	}
	return DecisionAllow
}

func (s *Service) calculateConfidence(results []RuleResult) decimal.Decimal {
	// Confidence is based on:
	// Number of rules that fired
	// Consistency of scores across rules
	// Quality of data (how many rules could be evaluated)

	if len(results) == 0 {
		return decimal.Zero
	}

	firedCount := 0
	for _, result := range results {
		if result.Fired {
			firedCount++
		}
	}

	// Simple confidence: ratio of fired rules to total rules
	// I believe this should be a sophisticated methods
	confidence := decimal.NewFromInt(int64(firedCount)).Div(decimal.NewFromInt(int64(len(results))))

	// Cap at 0.95 to account for uncertainty
	maxConfidence := decimal.NewFromFloat(0.95)
	if confidence.GreaterThan(maxConfidence) {
		return maxConfidence
	}

	return confidence
}

func (s *Service) createFraudCaseIfNeeded(ctx context.Context, evalCtx *RuleEvaluationContext, decision *FraudDecision) error {
	// Create case for high-risk decisions
	if decision.RiskLevel != RiskLevelHigh && decision.RiskLevel != RiskLevelCritical {
		return nil
	}

	description := "Automated fraud detection flagged transaction for review"
	_, err := s.CreateFraudCase(ctx, evalCtx.TransactionID, evalCtx.UserID, evalCtx.AccountID, decision.RiskLevel, description)
	return err
}

func (s *Service) validateRule(rule *Rule) error {
	if rule == nil {
		return ErrRuleConfigInvalid
	}

	// Validate rule type
	validTypes := map[RuleType]bool{
		RuleTypeVelocity:   true,
		RuleTypeAmount:     true,
		RuleTypeGeographic: true,
		RuleTypeDevice:     true,
		RuleTypeMerchant:   true,
		RuleTypeBehavioral: true,
	}
	if !validTypes[rule.Type] {
		return ErrInvalidRuleType
	}

	// Validate severity
	validSeverities := map[RuleSeverity]bool{
		SeverityLow:      true,
		SeverityMedium:   true,
		SeverityHigh:     true,
		SeverityCritical: true,
	}
	if !validSeverities[rule.Severity] {
		return ErrInvalidRuleSeverity
	}

	// Validate action
	validActions := map[RuleAction]bool{
		ActionAllow:     true,
		ActionBlock:     true,
		ActionReview:    true,
		ActionChallenge: true,
	}
	if !validActions[rule.Action] {
		return ErrInvalidRuleAction
	}

	// Validate config is not empty
	if len(rule.Config) == 0 {
		return ErrRuleConfigInvalid
	}

	return nil
}

func (s *Service) determineUserRiskLevel(profile *UserRiskProfile) RiskLevel {
	// Multiple factors determine user risk level
	score := 0.0

	// Blocked count (30%)
	if profile.BlockedCount >= 5 {
		score += 30.0
	} else if profile.BlockedCount >= 3 {
		score += 20.0
	} else if profile.BlockedCount >= 1 {
		score += 10.0
	}

	// Open cases (30%)
	if profile.OpenCasesCount >= 2 {
		score += 30.0
	} else if profile.OpenCasesCount >= 1 {
		score += 15.0
	}

	// Average risk score (40%)
	avgScore := profile.AverageRiskScore.InexactFloat64()
	score += avgScore * 40.0

	// Determine risk level from total score
	switch {
	case score >= 80.0:
		return RiskLevelCritical
	case score >= 60.0:
		return RiskLevelHigh
	case score >= 30.0:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

// SetDecisionThresholds allows customizing decision thresholds
func (s *Service) SetDecisionThresholds(thresholds DecisionThresholds) {
	s.decisionThresholds = thresholds
}

// SetScoreWeights allows customizing score weights
func (s *Service) SetScoreWeights(weights ScoreWeights) {
	s.scoreWeights = weights
}

// SetScoringStrategy allows customizing scoring strategy
func (s *Service) SetScoringStrategy(strategy ScoringStrategy) {
	s.scoringStrategy = strategy
}
