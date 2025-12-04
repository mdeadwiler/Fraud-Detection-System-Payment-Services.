package transaction 

import (
	"time"
	"fmt"
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"

	"fraud-detecction-system/internal/application/dto"
	"fraud-detecction-system/internal/domain/fraud"
	"fraud-detecction-system/internal/domain/transaction"
	)

// ProcessTransactionUseCase orchestrates transaction processing with fraud detection
// This is the main entry point for all incoming transactions 
type ProcessTransactionUseCase struct {
	// Domain services 
	txService *transaction.Service
	fraudService *fraud.Service

	// Configs
	fraudCheckTimeout time.Duration
	enableAsync bool
}

// NewProcessTransctionUseCase creates a new use case instance
func NewProcessTransctionUseCase(
	txService *transaction.Service,
	fraudeService *fraud.Service,
) *ProcessTransactionUseCase{
	return &ProcessTransactionUseCase{
		txService: txService,
		fraudService: fraudeService,
		fraudCheckTimeout: 200 * time.Millisecond, //p99 target
		enableAsync: false,  //Synchronous by default for correctness 
	}
}

// Execute processes a transaction with real-time fraud detection
// This is the critical path - optimized for sub-100ms p99 latency
func (uc *ProcessTransactionUseCase) Execute(
	ctx context.Context,
	req *dto.CreateTreansactionRequests,
) (*dto.TransactionResponse, error) {
	startTime := time.Now()

	// Convert DTO to domain entity
	tx := uc.mapRequestToTransaction(req)

	// Create transaction (status=pending)
	if err := uc.txService.CreateTransaction(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Run fraud detection with timeout
	fraudCtx, cancel := context.WithTimeout(ctx, uc.fraudCheckTimeout)
	defer cancel()

	fraudResult, err := uc.runFraudDetection(fraudCtx, tx, req)
	if err != nil {
		// Fraud check failed - flag for manual review as safety measure
		_ = uc.txService.FlagForReview(ctx, tx.ID, []string{"Fraud check timeout or error"}, decimal.Zero)
		return uc.buildResponse(tx, nil, time.Since(startTime)), fmt.Errorf("fraud check failed: %w", err)
	}

	// Apply fraud decision to transaction
	if err := uc.applyFraudDecision(ctx, tx.ID, fraudResult); err != nil {
		return nil, fmt.Errorf("failed to apply fraud decision: %w", err)
	}

	// Build response
	response := uc.buildResponse(tx, fraudResult, time.Since(startTime))
	return response, nil
}

// runFraudDetection performs fraud analysis on the transaction
// Uses parallel data fetching for optimal performance
func (uc *ProcessTransactionUseCase) runFraudDetection(
	ctx context.Context,
	tx *transaction.Transaction,
	req *dto.CreateTreansactionRequests,
) (*fraud.FraudDecision, error) {
	// Build evaluation context - fetch required data in parallel
	evalCtx, err := uc.buildEvaluationContext(ctx, tx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to build evaluation context: %w", err)
	}

	// Run fraud analysis
	decision, err := uc.fraudService.AnalyzeTransaction(ctx, evalCtx)
	if err != nil {
		return nil, fmt.Errorf("fraud analysis failed: %w", err)
	}

	return decision, nil
}

// buildEvaluationContext fetches all data needed for fraud rules
// OPTIMIZATION: Parallel fetching using errgroup for sub-100ms performance
func (uc *ProcessTransactionUseCase) buildEvaluationContext(
	ctx context.Context,
	tx *transaction.Transaction,
	req *dto.CreateTreansactionRequests,
) (*fraud.RuleEvaluationContext, error) {
	var (
		recentTxs     []*transaction.Transaction
		velocityCheck *transaction.VelocityCheckResult
	)

	// Use errgroup for concurrent data fetching
	g, gctx := errgroup.WithContext(ctx)

	// Fetch 1: Recent transactions (last 24h for behavioral analysis)
	g.Go(func() error {
		since := time.Now().Add(-24 * time.Hour)
		txs, err := uc.txService.GetRecentTransactions(gctx, tx.UserID, since)
		if err != nil {
			return fmt.Errorf("failed to fetch recent transactions: %w", err)
		}
		recentTxs = txs
		return nil
	})

	// Velocity check (Redis)
	g.Go(func() error {
		result, err := uc.txService.CheckVelocity(gctx, tx.UserID, tx.Amount)
		if err != nil {
			return fmt.Errorf("failed to check velocity: %w", err)
		}
		velocityCheck = result
		return nil
	})

	// Wait for all fetches to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Build evaluation context
	evalCtx := &fraud.RuleEvaluationContext{
		TransactionID: tx.ID,
		UserID:        tx.UserID,
		AccountID:     tx.AccountID,
		Amount:        tx.Amount,
		Currency:      string(tx.Currency),
		Timestamp:     tx.CreatedAt,

		// Context data
		Location: uc.mapGeoLocation(req.Location),
		Device:   uc.mapDeviceInfo(req.Device),
		Merchant: uc.mapMerchantInfo(req.Merchant),
		Payment:  uc.mapPaymentMethod(req.Payment),

		// Historical data
		RecentTransactions: uc.mapToTransactionSummaries(recentTxs),
		UserProfile:        uc.buildUserProfile(recentTxs, velocityCheck),
	}

	return evalCtx, nil
}

// applyFraudDecision updates transaction status based on fraud analysis
func (uc *ProcessTransactionUseCase) applyFraudDecision(
	ctx context.Context,
	txID uuid.UUID,
	decision *fraud.FraudDecision,
) error {
	switch decision.Decision {
	case fraud.DecisionAllow:
		// Approve transaction
		return uc.txService.ApproveTransaction(ctx, txID)

	case fraud.DecisionBlock:
		// Decline transaction with reasons
		return uc.txService.DeclineTransaction(ctx, txID, decision.Reasons)

	case fraud.DecisionReview:
		// Flag for manual review
		return uc.txService.FlagForReview(ctx, txID, decision.Reasons, decision.Score)

	case fraud.DecisionChallenge:
		// Flag for additional verification (3DS, OTP, etc.)
		return uc.txService.FlagForReview(ctx, txID, append(decision.Reasons, "Requires additional verification"), decision.Score)

	default:
		return fmt.Errorf("unknown fraud decision: %s", decision.Decision)
	}
}

// buildResponse constructs the API response
func (uc *ProcessTransactionUseCase) buildResponse(
	tx *transaction.Transaction,
	fraudDecision *fraud.FraudDecision,
	processingTime time.Duration,
) *dto.TransactionResponse {
	response := &dto.TransactionResponse{
		ID:               tx.ID,
		ExternalID:       tx.ExternalID,
		UserID:           tx.UserID,
		AccountID:        tx.AccountID,
		Type:             string(tx.Type),
		Status:           string(tx.Status),
		Amount:           tx.Amount,
		Currency:         string(tx.Currency),
		Description:      tx.Description,
		CreatedAt:        tx.CreatedAt,
		ProcessedAt:      tx.ProcessedAt,
		ProcessingTimeMs: processingTime.Milliseconds(),
	}

	// Add fraud analysis results if available
	if fraudDecision != nil {
		response.FraudScore = &fraudDecision.Score
		response.RiskLevel = string(fraudDecision.RiskLevel)
		response.FraudReasons = fraudDecision.Reasons
		response.RequiresReview = fraudDecision.Decision == fraud.DecisionReview || fraudDecision.Decision == fraud.DecisionChallenge
	}

	return response
}

// Mapping helpers
func (uc *ProcessTransactionUseCase) mapRequestToTransaction(req *dto.CreateTreansactionRequests) *transaction.Transaction {
	tx := transaction.NewTransaction(
		req.UserID,
		req.AccountID,
		transaction.TransactionType(req.Type),
		req.Amount,
		transaction.Currency(req.Currency),
	)

	tx.ExternalID = req.ExternalID
	tx.Description = req.Description
	tx.Metadata = req.Metadata

	// Map context data
	if req.Location != nil {
		tx.Location = &transaction.GeoLocation{
			Latitude:  req.Location.Latitude,
			Longitude: req.Location.Longitude,
			Country:   req.Location.Country,
			City:      req.Location.City,
			Region:    req.Location.Region,
			IPAddress: req.Location.IPAddress,
		}
	}

	if req.Device != nil {
		tx.Device = &transaction.DeviceInfo{
			DeviceID:        req.Device.DeviceID,
			DeviceType:      req.Device.Type,
			OS:              req.Device.OS,
			Browser:         req.Device.Browser,
			UserAgent:       req.Device.UserAgent,
			IsTrustedDevice: req.Device.TrustedDevice,
		}
	}

	if req.Merchant != nil {
		tx.Merchant = &transaction.MerchantInfo{
			MerchantID:       req.Merchant.MerchantID,
			MerchantName:     req.Merchant.Name,
			MerchantCategory: req.Merchant.Category,
			Country:          req.Merchant.Country,
		}
	}

	if req.Payment != nil {
		tx.Payment = &transaction.PaymentMethod{
			Type:           req.Payment.Type,
			Last4:          req.Payment.Last4,
			Network:        req.Payment.CardType,
			BankID:         req.Payment.BankID,
			IssuingCountry: req.Payment.IssuingCountry,
		}
	}

	return tx
}

func (uc *ProcessTransactionUseCase) mapGeoLocation(loc *dto.GeoLocation) *fraud.GeoLocation {
	if loc == nil {
		return nil
	}
	return &fraud.GeoLocation{
		Latitude:  loc.Latitude,
		Longitude: loc.Longitude,
		Country:   loc.Country,
		City:      loc.City,
		Region:    loc.Region,
		IPAddress: loc.IPAddress,
	}
}

func (uc *ProcessTransactionUseCase) mapDeviceInfo(dto *dto.DeviceInfoDTO) *fraud.DeviceInfo {
	if dto == nil {
		return nil
	}
	return &fraud.DeviceInfo{
		DeviceID:        dto.DeviceID,
		DeviceType:      dto.Type,
		OS:              dto.OS,
		Browser:         dto.Browser,
		UserAgent:       dto.UserAgent,
		IsTrustedDevice: dto.TrustedDevice,
	}
}

func (uc *ProcessTransactionUseCase) mapMerchantInfo(dto *dto.MerchantDTO) *fraud.MerchantInfo {
	if dto == nil {
		return nil
	}
	return &fraud.MerchantInfo{
		MerchantID:       dto.MerchantID,
		MerchantName:     dto.Name,
		MerchantCategory: dto.Category,
		Country:          dto.Country,
	}
}

func (uc *ProcessTransactionUseCase) mapPaymentMethod(dto *dto.PaymentDTO) *fraud.PaymentMethod {
	if dto == nil {
		return nil
	}
	return &fraud.PaymentMethod{
		Type:           dto.Type,
		Last4:          dto.Last4,
		Network:        dto.CardType,
		BankID:         dto.BankID,
		IssuingCountry: dto.IssuingCountry,
	}
}

func (uc *ProcessTransactionUseCase) mapToTransactionSummaries(txs []*transaction.Transaction) []fraud.TransactionSummary {
	summaries := make([]fraud.TransactionSummary, len(txs))
	for i, tx := range txs {
		summaries[i] = fraud.TransactionSummary{
			ID:        tx.ID,
			Amount:    tx.Amount,
			Timestamp: tx.CreatedAt,
			Location:  uc.mapGeoLocationFromTx(tx.Location),
			Status:    string(tx.Status),
		}
	}
	return summaries
}

func (uc *ProcessTransactionUseCase) mapGeoLocationFromTx(loc *transaction.GeoLocation) *fraud.GeoLocation {
	if loc == nil {
		return nil
	}
	return &fraud.GeoLocation{
		Latitude:  loc.Latitude,
		Longitude: loc.Longitude,
		Country:   loc.Country,
		City:      loc.City,
		Region:    loc.Region,
		IPAddress: loc.IPAddress,
	}
}

func (uc *ProcessTransactionUseCase) buildUserProfile(
	recentTxs []*transaction.Transaction,
	velocityCheck *transaction.VelocityCheckResult,
) *fraud.UserProfile {
	if len(recentTxs) == 0 {
		return nil
	}

	// Calculate average transaction amount
	total := decimal.Zero
	locations := make(map[string]bool)
	for _, tx := range recentTxs {
		total = total.Add(tx.Amount)
		if tx.Location != nil {
			locations[tx.Location.Country] = true
		}
	}
	avgAmount := total.Div(decimal.NewFromInt(int64(len(recentTxs))))

	// Extract typical locations
	typicalLocations := make([]string, 0, len(locations))
	for loc := range locations {
		typicalLocations = append(typicalLocations, loc)
	}

	return &fraud.UserProfile{
		UserID:             recentTxs[0].UserID,
		AverageTransaction: avgAmount,
		TypicalLocations:   typicalLocations,
		LastActivityAt:     time.Now(),
	}
}

