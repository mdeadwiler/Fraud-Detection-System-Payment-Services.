package rules

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fraud-detecction-system/internal/domain/fraud"
	"fraud-detecction-system/internal/infrastructure/cache/redis"
)

// Engine implements fraud.RuleEngine
type Engine struct {
	ruleRepo      fraud.RuleRepository
	velocityCache *redis.VelocityCache
	deviceCache   *redis.DeviceCache
	locationCache *redis.LocationCache

	// In-memory rule cache for performance
	rulesCache []*fraud.Rule
	rulesMu    sync.RWMutex
	lastRefresh time.Time
	cacheTTL   time.Duration
}

// NewEngine creates a new rule engine
func NewEngine(
	ruleRepo fraud.RuleRepository,
	velocityCache *redis.VelocityCache,
	deviceCache *redis.DeviceCache,
	locationCache *redis.LocationCache,
) *Engine {
	return &Engine{
		ruleRepo:      ruleRepo,
		velocityCache: velocityCache,
		deviceCache:   deviceCache,
		locationCache: locationCache,
		cacheTTL:      5 * time.Minute,
	}
}

// Evaluate runs all enabled rules against a transaction context
func (e *Engine) Evaluate(ctx context.Context, evalCtx *fraud.RuleEvaluationContext) ([]fraud.RuleResult, error) {
	rules, err := e.GetActiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active rules: %w", err)
	}

	results := make([]fraud.RuleResult, 0, len(rules))

	for _, rule := range rules {
		result, err := e.EvaluateRule(ctx, rule, evalCtx)
		if err != nil {
			// Log error but continue with other rules
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}

// EvaluateRule runs a specific rule
func (e *Engine) EvaluateRule(ctx context.Context, rule *fraud.Rule, evalCtx *fraud.RuleEvaluationContext) (*fraud.RuleResult, error) {
	if !rule.IsActive() {
		return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Rule not active", fraud.ActionAllow), nil
	}

	switch rule.Type {
	case fraud.RuleTypeVelocity:
		return e.evaluateVelocityRule(ctx, rule, evalCtx)
	case fraud.RuleTypeAmount:
		return e.evaluateAmountRule(ctx, rule, evalCtx)
	case fraud.RuleTypeGeographic:
		return e.evaluateGeographicRule(ctx, rule, evalCtx)
	case fraud.RuleTypeDevice:
		return e.evaluateDeviceRule(ctx, rule, evalCtx)
	case fraud.RuleTypeMerchant:
		return e.evaluateMerchantRule(ctx, rule, evalCtx)
	case fraud.RuleTypeBehavioral:
		return e.evaluateBehavioralRule(ctx, rule, evalCtx)
	default:
		return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Unknown rule type", fraud.ActionAllow), nil
	}
}

// GetActiveRules retrieves all currently active rules
func (e *Engine) GetActiveRules(ctx context.Context) ([]*fraud.Rule, error) {
	e.rulesMu.RLock()
	if e.rulesCache != nil && time.Since(e.lastRefresh) < e.cacheTTL {
		rules := e.rulesCache
		e.rulesMu.RUnlock()
		return rules, nil
	}
	e.rulesMu.RUnlock()

	// Refresh cache
	e.rulesMu.Lock()
	defer e.rulesMu.Unlock()

	// Double-check after acquiring write lock
	if e.rulesCache != nil && time.Since(e.lastRefresh) < e.cacheTTL {
		return e.rulesCache, nil
	}

	rules, err := e.ruleRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	e.rulesCache = rules
	e.lastRefresh = time.Now()
	return rules, nil
}

// AddRule adds a new rule to the engine
func (e *Engine) AddRule(ctx context.Context, rule *fraud.Rule) error {
	if err := e.ruleRepo.Create(ctx, rule); err != nil {
		return err
	}
	e.invalidateCache()
	return nil
}

// UpdateRule updates an existing rule
func (e *Engine) UpdateRule(ctx context.Context, rule *fraud.Rule) error {
	if err := e.ruleRepo.Update(ctx, rule); err != nil {
		return err
	}
	e.invalidateCache()
	return nil
}

// DisableRule disables a rule
func (e *Engine) DisableRule(ctx context.Context, ruleID uuid.UUID) error {
	if err := e.ruleRepo.Disable(ctx, ruleID); err != nil {
		return err
	}
	e.invalidateCache()
	return nil
}

func (e *Engine) invalidateCache() {
	e.rulesMu.Lock()
	e.rulesCache = nil
	e.rulesMu.Unlock()
}

// evaluateVelocityRule checks transaction frequency limits
func (e *Engine) evaluateVelocityRule(ctx context.Context, rule *fraud.Rule, evalCtx *fraud.RuleEvaluationContext) (*fraud.RuleResult, error) {
	config := parseVelocityConfig(rule.Config)

	windowDuration := time.Duration(config.WindowMinutes) * time.Minute

	// Get transaction count in window
	count, err := e.velocityCache.GetTransactionCount(ctx, evalCtx.UserID, windowDuration)
	if err != nil {
		// Can't evaluate velocity - fail open for availability
		return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Unable to check velocity", fraud.ActionAllow), nil
	}

	// Check if count exceeds limit
	if count >= int64(config.MaxTransactions) {
		score := calculateVelocityScore(count, config.MaxTransactions)
		reason := fmt.Sprintf("Velocity limit exceeded: %d transactions in %d minutes (limit: %d)", count, config.WindowMinutes, config.MaxTransactions)
		result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, rule.Action)
		result.AddMetadata("transaction_count", count)
		result.AddMetadata("limit", config.MaxTransactions)
		result.AddMetadata("window_minutes", config.WindowMinutes)
		return result, nil
	}

	// If amount threshold is configured, also check total amount
	if !config.AmountThreshold.IsZero() && !config.CountOnly {
		total, err := e.velocityCache.GetTransactionSum(ctx, evalCtx.UserID, windowDuration)
		if err == nil && total.Add(evalCtx.Amount).GreaterThan(config.AmountThreshold) {
			score := decimal.NewFromFloat(0.7)
			reason := fmt.Sprintf("Amount velocity limit exceeded: %s total in %d minutes (limit: %s)", total.Add(evalCtx.Amount).String(), config.WindowMinutes, config.AmountThreshold.String())
			result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, rule.Action)
			result.AddMetadata("total_amount", total.String())
			result.AddMetadata("amount_limit", config.AmountThreshold.String())
			return result, nil
		}
	}

	return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Within velocity limits", fraud.ActionAllow), nil
}

// evaluateAmountRule checks transaction amount thresholds
func (e *Engine) evaluateAmountRule(ctx context.Context, rule *fraud.Rule, evalCtx *fraud.RuleEvaluationContext) (*fraud.RuleResult, error) {
	config := parseAmountConfig(rule.Config)

	// Check max amount
	if !config.MaxAmount.IsZero() && evalCtx.Amount.GreaterThan(config.MaxAmount) {
		score := calculateAmountScore(evalCtx.Amount, config.MaxAmount)
		reason := fmt.Sprintf("Transaction amount %s exceeds maximum threshold %s", evalCtx.Amount.String(), config.MaxAmount.String())
		result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, rule.Action)
		result.AddMetadata("amount", evalCtx.Amount.String())
		result.AddMetadata("max_amount", config.MaxAmount.String())
		return result, nil
	}

	// Check deviation from user's average
	if config.DeviationFactor > 0 && evalCtx.UserProfile != nil {
		avgAmount := evalCtx.UserProfile.AverageTransaction
		if !avgAmount.IsZero() {
			threshold := avgAmount.Mul(decimal.NewFromFloat(config.DeviationFactor))
			if evalCtx.Amount.GreaterThan(threshold) {
				score := decimal.NewFromFloat(0.65)
				reason := fmt.Sprintf("Transaction amount %s is %.1fx user's average (%s)", evalCtx.Amount.String(), config.DeviationFactor, avgAmount.String())
				result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, rule.Action)
				result.AddMetadata("amount", evalCtx.Amount.String())
				result.AddMetadata("average", avgAmount.String())
				result.AddMetadata("deviation_factor", config.DeviationFactor)
				return result, nil
			}
		}
	}

	return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Amount within limits", fraud.ActionAllow), nil
}

// evaluateGeographicRule checks location-based rules
func (e *Engine) evaluateGeographicRule(ctx context.Context, rule *fraud.Rule, evalCtx *fraud.RuleEvaluationContext) (*fraud.RuleResult, error) {
	if evalCtx.Location == nil {
		return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "No location data", fraud.ActionAllow), nil
	}

	config := parseGeographicConfig(rule.Config)

	// Check blocked countries
	for _, blocked := range config.BlockedCountries {
		if evalCtx.Location.Country == blocked {
			score := decimal.NewFromFloat(0.9)
			reason := fmt.Sprintf("Transaction from blocked country: %s", evalCtx.Location.Country)
			result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionBlock)
			result.AddMetadata("country", evalCtx.Location.Country)
			return result, nil
		}
	}

	// Check allowed countries (if configured)
	if len(config.AllowedCountries) > 0 {
		allowed := false
		for _, c := range config.AllowedCountries {
			if evalCtx.Location.Country == c {
				allowed = true
				break
			}
		}
		if !allowed {
			score := decimal.NewFromFloat(0.75)
			reason := fmt.Sprintf("Transaction from non-allowed country: %s", evalCtx.Location.Country)
			result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, rule.Action)
			result.AddMetadata("country", evalCtx.Location.Country)
			return result, nil
		}
	}

	// Check if location is known for this user
	if config.RequireConsistent && e.locationCache != nil {
		isKnown, err := e.locationCache.IsKnownLocation(ctx, evalCtx.UserID, evalCtx.Location.Country, evalCtx.Location.City)
		if err == nil && !isKnown {
			score := decimal.NewFromFloat(0.5)
			reason := fmt.Sprintf("Transaction from new location: %s, %s", evalCtx.Location.City, evalCtx.Location.Country)
			result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionChallenge)
			result.AddMetadata("city", evalCtx.Location.City)
			result.AddMetadata("country", evalCtx.Location.Country)
			return result, nil
		}
	}

	// Check distance from last known location (simplified - would need actual geocalc)
	if config.MaxDistanceKm > 0 && len(evalCtx.RecentTransactions) > 0 {
		lastTx := evalCtx.RecentTransactions[0]
		if lastTx.Location != nil {
			distance := haversineDistance(
				evalCtx.Location.Latitude, evalCtx.Location.Longitude,
				lastTx.Location.Latitude, lastTx.Location.Longitude,
			)
			if distance > config.MaxDistanceKm {
				// Check time between transactions (impossible travel)
				timeDiff := evalCtx.Timestamp.Sub(lastTx.Timestamp)
				speedKmH := distance / timeDiff.Hours()
				if speedKmH > 900 { // Faster than a commercial jet
					score := decimal.NewFromFloat(0.85)
					reason := fmt.Sprintf("Impossible travel: %.0fkm in %v (%.0f km/h)", distance, timeDiff, speedKmH)
					result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionBlock)
					result.AddMetadata("distance_km", distance)
					result.AddMetadata("speed_kmh", speedKmH)
					return result, nil
				}
			}
		}
	}

	return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Location check passed", fraud.ActionAllow), nil
}

// evaluateDeviceRule checks device-related rules
func (e *Engine) evaluateDeviceRule(ctx context.Context, rule *fraud.Rule, evalCtx *fraud.RuleEvaluationContext) (*fraud.RuleResult, error) {
	if evalCtx.Device == nil {
		return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "No device data", fraud.ActionAllow), nil
	}

	config := parseDeviceConfig(rule.Config)

	// Check if device is trusted
	if config.RequireTrustedDevice && !evalCtx.Device.IsTrustedDevice {
		// Check if it's a known device
		if e.deviceCache != nil {
			isKnown, err := e.deviceCache.IsKnownDevice(ctx, evalCtx.UserID, evalCtx.Device.DeviceID)
			if err == nil && !isKnown {
				if config.BlockNewDevices {
					score := decimal.NewFromFloat(0.8)
					reason := "Transaction from untrusted, unknown device"
					result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionBlock)
					result.AddMetadata("device_id", evalCtx.Device.DeviceID)
					return result, nil
				}
				score := decimal.NewFromFloat(0.55)
				reason := "Transaction from new device"
				result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionChallenge)
				result.AddMetadata("device_id", evalCtx.Device.DeviceID)
				return result, nil
			}
		}
	}

	// Check device count per user
	if config.MaxDevicesPerUser > 0 && e.deviceCache != nil {
		deviceCount, err := e.deviceCache.GetDeviceCount(ctx, evalCtx.UserID)
		if err == nil && int(deviceCount) >= config.MaxDevicesPerUser {
			// Check if this is a new device
			isKnown, _ := e.deviceCache.IsKnownDevice(ctx, evalCtx.UserID, evalCtx.Device.DeviceID)
			if !isKnown {
				score := decimal.NewFromFloat(0.6)
				reason := fmt.Sprintf("User has %d devices (limit: %d) and this is a new device", deviceCount, config.MaxDevicesPerUser)
				result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, rule.Action)
				result.AddMetadata("device_count", deviceCount)
				result.AddMetadata("max_devices", config.MaxDevicesPerUser)
				return result, nil
			}
		}
	}

	return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Device check passed", fraud.ActionAllow), nil
}

// evaluateMerchantRule checks merchant-related rules
func (e *Engine) evaluateMerchantRule(ctx context.Context, rule *fraud.Rule, evalCtx *fraud.RuleEvaluationContext) (*fraud.RuleResult, error) {
	if evalCtx.Merchant == nil {
		return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "No merchant data", fraud.ActionAllow), nil
	}

	// High-risk merchant check
	if evalCtx.Merchant.IsHighRisk {
		score := decimal.NewFromFloat(0.45)
		reason := fmt.Sprintf("Transaction with high-risk merchant: %s", evalCtx.Merchant.MerchantName)
		result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionReview)
		result.AddMetadata("merchant_name", evalCtx.Merchant.MerchantName)
		result.AddMetadata("merchant_category", evalCtx.Merchant.MerchantCategory)
		return result, nil
	}

	// High-risk MCC codes (gambling, crypto, etc.)
	highRiskMCCs := map[string]bool{
		"7995": true, // Gambling
		"7801": true, // Lottery
		"5967": true, // Direct Marketing
		"6051": true, // Crypto
	}

	if highRiskMCCs[evalCtx.Merchant.MerchantCategory] {
		score := decimal.NewFromFloat(0.4)
		reason := fmt.Sprintf("Transaction with high-risk merchant category: %s", evalCtx.Merchant.MerchantCategory)
		result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionReview)
		result.AddMetadata("merchant_category", evalCtx.Merchant.MerchantCategory)
		return result, nil
	}

	return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Merchant check passed", fraud.ActionAllow), nil
}

// evaluateBehavioralRule checks behavioral patterns
func (e *Engine) evaluateBehavioralRule(ctx context.Context, rule *fraud.Rule, evalCtx *fraud.RuleEvaluationContext) (*fraud.RuleResult, error) {
	if evalCtx.UserProfile == nil {
		return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "No user profile", fraud.ActionAllow), nil
	}

	// Check for unusual timing (outside user's typical activity hours)
	hour := evalCtx.Timestamp.Hour()
	if hour >= 2 && hour <= 5 { // 2 AM - 5 AM is unusual
		score := decimal.NewFromFloat(0.35)
		reason := "Transaction at unusual hour (late night)"
		result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionChallenge)
		result.AddMetadata("hour", hour)
		return result, nil
	}

	// Check account age - new accounts are higher risk
	if evalCtx.UserProfile.AccountAge < 24*time.Hour {
		score := decimal.NewFromFloat(0.5)
		reason := "Transaction from very new account (< 24 hours)"
		result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionReview)
		result.AddMetadata("account_age_hours", evalCtx.UserProfile.AccountAge.Hours())
		return result, nil
	}

	// Dormant account suddenly active
	if evalCtx.UserProfile.LastActivityAt.Before(time.Now().AddDate(0, -3, 0)) {
		score := decimal.NewFromFloat(0.55)
		reason := "Transaction from dormant account (inactive > 3 months)"
		result := fraud.NewRuleResult(rule.ID, rule.Name, true, score, reason, fraud.ActionReview)
		result.AddMetadata("last_activity", evalCtx.UserProfile.LastActivityAt.Format(time.RFC3339))
		return result, nil
	}

	return fraud.NewRuleResult(rule.ID, rule.Name, false, decimal.Zero, "Behavioral check passed", fraud.ActionAllow), nil
}

// Helper functions

func calculateVelocityScore(count int64, limit int) decimal.Decimal {
	ratio := float64(count) / float64(limit)
	if ratio >= 2.0 {
		return decimal.NewFromFloat(0.9)
	}
	return decimal.NewFromFloat(0.5 + (ratio-1.0)*0.4)
}

func calculateAmountScore(amount, limit decimal.Decimal) decimal.Decimal {
	ratio := amount.Div(limit).InexactFloat64()
	if ratio >= 5.0 {
		return decimal.NewFromFloat(0.95)
	}
	if ratio >= 2.0 {
		return decimal.NewFromFloat(0.8)
	}
	return decimal.NewFromFloat(0.6 + (ratio-1.0)*0.2)
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0 // km

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// Config parsers

func parseVelocityConfig(config map[string]interface{}) fraud.VelocityRuleConfig {
	result := fraud.VelocityRuleConfig{
		MaxTransactions: 10,
		WindowMinutes:   5,
	}

	if v, ok := config["max_transactions"].(float64); ok {
		result.MaxTransactions = int(v)
	}
	if v, ok := config["window_minutes"].(float64); ok {
		result.WindowMinutes = int(v)
	}
	if v, ok := config["amount_threshold"].(string); ok {
		result.AmountThreshold, _ = decimal.NewFromString(v)
	}
	if v, ok := config["count_only"].(bool); ok {
		result.CountOnly = v
	}

	return result
}

func parseAmountConfig(config map[string]interface{}) fraud.AmountRuleConfig {
	result := fraud.AmountRuleConfig{}

	if v, ok := config["min_amount"].(string); ok {
		result.MinAmount, _ = decimal.NewFromString(v)
	}
	if v, ok := config["max_amount"].(string); ok {
		result.MaxAmount, _ = decimal.NewFromString(v)
	}
	if v, ok := config["deviation_factor"].(float64); ok {
		result.DeviationFactor = v
	}

	return result
}

func parseGeographicConfig(config map[string]interface{}) fraud.GeographicRuleConfig {
	result := fraud.GeographicRuleConfig{}

	if v, ok := config["allowed_countries"].([]interface{}); ok {
		for _, c := range v {
			if s, ok := c.(string); ok {
				result.AllowedCountries = append(result.AllowedCountries, s)
			}
		}
	}
	if v, ok := config["blocked_countries"].([]interface{}); ok {
		for _, c := range v {
			if s, ok := c.(string); ok {
				result.BlockedCountries = append(result.BlockedCountries, s)
			}
		}
	}
	if v, ok := config["max_distance_km"].(float64); ok {
		result.MaxDistanceKm = v
	}
	if v, ok := config["require_consistent"].(bool); ok {
		result.RequireConsistent = v
	}

	return result
}

func parseDeviceConfig(config map[string]interface{}) fraud.DeviceRuleConfig {
	result := fraud.DeviceRuleConfig{}

	if v, ok := config["require_trusted_device"].(bool); ok {
		result.RequireTrustedDevice = v
	}
	if v, ok := config["max_devices_per_user"].(float64); ok {
		result.MaxDevicesPerUser = int(v)
	}
	if v, ok := config["block_new_devices"].(bool); ok {
		result.BlockNewDevices = v
	}

	return result
}

