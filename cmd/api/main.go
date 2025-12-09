package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	fraudapp "fraud-detecction-system/internal/application/fraud"
	"fraud-detecction-system/internal/domain/fraud"
	"fraud-detecction-system/internal/infrastructure/cache/redis"
	"fraud-detecction-system/internal/infrastructure/database/postgres"
	"fraud-detecction-system/internal/infrastructure/http/router"
	"fraud-detecction-system/internal/infrastructure/ml"
	"fraud-detecction-system/internal/infrastructure/rules"
	"fraud-detecction-system/internal/interfaces/http/handler"
	"fraud-detecction-system/internal/pkg/config"
)

const version = "1.0.0"

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: Could not load config file, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	log.Printf("Starting Fraud Detection API v%s", version)
	log.Printf("Server will listen on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Initialize dependencies
	ctx := context.Background()

	// Database connection
	var dbClient *postgres.Client
	var decisionRepo *postgres.DecisionRepository
	var caseRepo *postgres.CaseRepository
	var ruleRepo *postgres.RuleRepository

	dbClient, err = postgres.NewClient(postgres.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Name,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		log.Printf("Warning: Database connection failed (running in limited mode): %v", err)
		dbClient = nil
	} else {
		log.Printf("Connected to PostgreSQL at %s:%d", cfg.Database.Host, cfg.Database.Port)
		decisionRepo = postgres.NewDecisionRepository(dbClient)
		caseRepo = postgres.NewCaseRepository(dbClient)
		ruleRepo = postgres.NewRuleRepository(dbClient)
	}

	// Redis connection
	var redisClient *redis.Client
	var velocityCache *redis.VelocityCache
	var deviceCache *redis.DeviceCache
	var locationCache *redis.LocationCache

	redisClient, err = redis.NewClient(redis.Config{
		Host:         cfg.Redis.Host,
		Port:         cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err != nil {
		log.Printf("Warning: Redis connection failed (velocity checks disabled): %v", err)
		redisClient = nil
	} else {
		log.Printf("Connected to Redis at %s:%d", cfg.Redis.Host, cfg.Redis.Port)
		velocityCache = redis.NewVelocityCache(redisClient)
		deviceCache = redis.NewDeviceCache(redisClient)
		locationCache = redis.NewLocationCache(redisClient)
	}

	// Initialize rule engine
	var ruleEngine *rules.Engine
	if ruleRepo != nil {
		ruleEngine = rules.NewEngine(ruleRepo, velocityCache, deviceCache, locationCache)
	} else {
		// Create a mock rule repository for standalone mode
		ruleEngine = rules.NewEngine(NewMockRuleRepository(), velocityCache, deviceCache, locationCache)
	}

	// Initialize ML predictor
	featureExtractor := ml.NewFeatureExtractor(
		cfg.Fraud.HighValueThreshold,
		cfg.Fraud.BlockedCountries,
	)
	mlPredictor := ml.NewPredictor(featureExtractor, cfg.ML.ModelVersion, cfg.ML.Enabled)

	// Initialize fraud service
	var fraudService *fraud.Service
	if decisionRepo != nil && caseRepo != nil && ruleRepo != nil {
		fraudService = fraud.NewService(decisionRepo, caseRepo, ruleRepo, ruleEngine, nil)
	} else {
		// Create with mock repositories for standalone mode
		fraudService = fraud.NewService(
			NewMockDecisionRepository(),
			NewMockCaseRepository(),
			NewMockRuleRepository(),
			ruleEngine,
			nil,
		)
	}

	// Set custom thresholds
	fraudService.SetDecisionThresholds(fraud.DecisionThresholds{
		BlockThreshold:     decimal.NewFromFloat(cfg.Fraud.BlockThreshold),
		ReviewThreshold:    decimal.NewFromFloat(cfg.Fraud.ReviewThreshold),
		ChallengeThreshold: decimal.NewFromFloat(cfg.Fraud.ChallengeThreshold),
	})

	fraudService.SetScoreWeights(fraud.ScoreWeights{
		Velocity:   decimal.NewFromFloat(cfg.Fraud.VelocityWeight),
		Amount:     decimal.NewFromFloat(cfg.Fraud.AmountWeight),
		Geographic: decimal.NewFromFloat(cfg.Fraud.GeographicWeight),
		Device:     decimal.NewFromFloat(cfg.Fraud.DeviceWeight),
		Merchant:   decimal.NewFromFloat(cfg.Fraud.MerchantWeight),
		Behavioral: decimal.NewFromFloat(cfg.Fraud.BehavioralWeight),
		MLModel:    decimal.NewFromFloat(cfg.Fraud.MLWeight),
	})

	// Initialize use case
	detectFraudUseCase := fraudapp.NewDetectFraudUseCase(
		fraudService,
		ruleEngine,
		mlPredictor,
		velocityCache,
		deviceCache,
		locationCache,
		cfg.Fraud.AnalysisTimeout,
	)

	// Initialize handlers
	fraudHandler := handler.NewFraudHandler(detectFraudUseCase, fraudService)

	var dbHealthChecker handler.HealthChecker
	var redisHealthChecker handler.HealthChecker
	if dbClient != nil {
		dbHealthChecker = dbClient
	}
	if redisClient != nil {
		redisHealthChecker = redisClient
	}
	healthHandler := handler.NewHealthHandler(dbHealthChecker, redisHealthChecker, version)

	// Create router
	r := router.NewRouter(fraudHandler, healthHandler)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("HTTP server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(ctx, cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Close connections
	if dbClient != nil {
		dbClient.Close()
	}
	if redisClient != nil {
		redisClient.Close()
	}

	log.Println("Server stopped")
}

// Mock repositories for standalone mode (when DB is not available)

// MockDecisionRepository implements fraud.DecisionRepository for standalone mode
type MockDecisionRepository struct {
	decisions map[string]*fraud.FraudDecision
}

func NewMockDecisionRepository() *MockDecisionRepository {
	return &MockDecisionRepository{
		decisions: make(map[string]*fraud.FraudDecision),
	}
}

func (r *MockDecisionRepository) Create(ctx context.Context, decision *fraud.FraudDecision) error {
	r.decisions[decision.ID.String()] = decision
	return nil
}

func (r *MockDecisionRepository) GetByID(ctx context.Context, id uuid.UUID) (*fraud.FraudDecision, error) {
	if d, ok := r.decisions[id.String()]; ok {
		return d, nil
	}
	return nil, fraud.ErrDecisionNotFound
}

func (r *MockDecisionRepository) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*fraud.FraudDecision, error) {
	for _, d := range r.decisions {
		if d.TransactionID == transactionID {
			return d, nil
		}
	}
	return nil, fraud.ErrDecisionNotFound
}

func (r *MockDecisionRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*fraud.FraudDecision, error) {
	var results []*fraud.FraudDecision
	for _, d := range r.decisions {
		if d.UserID == userID {
			results = append(results, d)
		}
	}
	return results, nil
}

func (r *MockDecisionRepository) GetBlockedCount(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	var count int64
	for _, d := range r.decisions {
		if d.UserID == userID && d.Decision == fraud.DecisionBlock && d.CreatedAt.After(since) {
			count++
		}
	}
	return count, nil
}

// MockCaseRepository implements fraud.CaseRepository for standalone mode
type MockCaseRepository struct {
	cases map[string]*fraud.FraudCase
}

func NewMockCaseRepository() *MockCaseRepository {
	return &MockCaseRepository{
		cases: make(map[string]*fraud.FraudCase),
	}
}

func (r *MockCaseRepository) Create(ctx context.Context, fraudCase *fraud.FraudCase) error {
	r.cases[fraudCase.ID.String()] = fraudCase
	return nil
}

func (r *MockCaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*fraud.FraudCase, error) {
	if c, ok := r.cases[id.String()]; ok {
		return c, nil
	}
	return nil, fraud.ErrCaseNotFound
}

func (r *MockCaseRepository) Update(ctx context.Context, fraudCase *fraud.FraudCase) error {
	r.cases[fraudCase.ID.String()] = fraudCase
	return nil
}

func (r *MockCaseRepository) ListByStatus(ctx context.Context, status fraud.CaseStatus, limit, offset int) ([]*fraud.FraudCase, error) {
	var results []*fraud.FraudCase
	for _, c := range r.cases {
		if c.Status == status {
			results = append(results, c)
		}
	}
	return results, nil
}

func (r *MockCaseRepository) ListByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*fraud.FraudCase, error) {
	var results []*fraud.FraudCase
	for _, c := range r.cases {
		if c.AssignedTo != nil && *c.AssignedTo == assigneeID {
			results = append(results, c)
		}
	}
	return results, nil
}

func (r *MockCaseRepository) GetOpenCasesByUser(ctx context.Context, userID uuid.UUID) ([]*fraud.FraudCase, error) {
	var results []*fraud.FraudCase
	for _, c := range r.cases {
		if c.UserID == userID && c.IsOpen() {
			results = append(results, c)
		}
	}
	return results, nil
}

// MockRuleRepository implements fraud.RuleRepository for standalone mode
type MockRuleRepository struct {
	rules map[string]*fraud.Rule
}

func NewMockRuleRepository() *MockRuleRepository {
	repo := &MockRuleRepository{
		rules: make(map[string]*fraud.Rule),
	}
	// Add default rules
	repo.seedDefaultRules()
	return repo
}

func (r *MockRuleRepository) seedDefaultRules() {
	// Velocity rule
	velocityRule := fraud.NewRule(
		"high_velocity",
		"Block if more than 5 transactions in 5 minutes",
		fraud.RuleTypeVelocity,
		fraud.SeverityHigh,
		fraud.ActionBlock,
		uuid.Nil,
	)
	velocityRule.Config = map[string]interface{}{
		"max_transactions": float64(5),
		"window_minutes":   float64(5),
	}
	r.rules[velocityRule.ID.String()] = velocityRule

	// Amount rule
	amountRule := fraud.NewRule(
		"high_amount",
		"Review transactions over $5000",
		fraud.RuleTypeAmount,
		fraud.SeverityMedium,
		fraud.ActionReview,
		uuid.Nil,
	)
	amountRule.Config = map[string]interface{}{
		"max_amount":       "5000",
		"deviation_factor": float64(5),
	}
	r.rules[amountRule.ID.String()] = amountRule

	// Geographic rule
	geoRule := fraud.NewRule(
		"blocked_countries",
		"Block transactions from high-risk countries",
		fraud.RuleTypeGeographic,
		fraud.SeverityCritical,
		fraud.ActionBlock,
		uuid.Nil,
	)
	geoRule.Config = map[string]interface{}{
		"blocked_countries":  []interface{}{"KP", "IR", "SY"},
		"require_consistent": true,
	}
	r.rules[geoRule.ID.String()] = geoRule

	// Device rule
	deviceRule := fraud.NewRule(
		"new_device",
		"Challenge transactions from new devices",
		fraud.RuleTypeDevice,
		fraud.SeverityMedium,
		fraud.ActionChallenge,
		uuid.Nil,
	)
	deviceRule.Config = map[string]interface{}{
		"require_trusted_device": true,
		"max_devices_per_user":   float64(5),
	}
	r.rules[deviceRule.ID.String()] = deviceRule

	// Behavioral rule
	behaviorRule := fraud.NewRule(
		"unusual_time",
		"Review transactions at unusual times",
		fraud.RuleTypeBehavioral,
		fraud.SeverityLow,
		fraud.ActionReview,
		uuid.Nil,
	)
	behaviorRule.Config = map[string]interface{}{}
	r.rules[behaviorRule.ID.String()] = behaviorRule
}

func (r *MockRuleRepository) Create(ctx context.Context, rule *fraud.Rule) error {
	r.rules[rule.ID.String()] = rule
	return nil
}

func (r *MockRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*fraud.Rule, error) {
	if rule, ok := r.rules[id.String()]; ok {
		return rule, nil
	}
	return nil, fraud.ErrRuleNotFound
}

func (r *MockRuleRepository) Update(ctx context.Context, rule *fraud.Rule) error {
	r.rules[rule.ID.String()] = rule
	return nil
}

func (r *MockRuleRepository) ListActive(ctx context.Context) ([]*fraud.Rule, error) {
	var results []*fraud.Rule
	for _, rule := range r.rules {
		if rule.IsActive() {
			results = append(results, rule)
		}
	}
	return results, nil
}

func (r *MockRuleRepository) ListByType(ctx context.Context, ruleType fraud.RuleType) ([]*fraud.Rule, error) {
	var results []*fraud.Rule
	for _, rule := range r.rules {
		if rule.Type == ruleType && rule.Enabled {
			results = append(results, rule)
		}
	}
	return results, nil
}

func (r *MockRuleRepository) Disable(ctx context.Context, ruleID uuid.UUID) error {
	if rule, ok := r.rules[ruleID.String()]; ok {
		rule.Disable()
		return nil
	}
	return fraud.ErrRuleNotFound
}

func (r *MockRuleRepository) GetVersion(ctx context.Context, ruleID uuid.UUID, version int) (*fraud.Rule, error) {
	return r.GetByID(ctx, ruleID)
}

