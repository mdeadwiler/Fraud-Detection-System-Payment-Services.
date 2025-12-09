package ml

import (
	"context"
	"math"
	"time"

	"github.com/shopspring/decimal"

	"fraud-detecction-system/internal/domain/fraud"
)

// Features represents the feature vector for ML prediction
type Features struct {
	// Transaction features
	Amount              float64 `json:"amount"`
	AmountLog           float64 `json:"amount_log"`
	IsHighValue         float64 `json:"is_high_value"` // 0 or 1
	Currency            string  `json:"currency"`

	// Time features
	HourOfDay           int     `json:"hour_of_day"`
	DayOfWeek           int     `json:"day_of_week"`
	IsWeekend           float64 `json:"is_weekend"` // 0 or 1
	IsNightTime         float64 `json:"is_night_time"` // 0 or 1

	// Velocity features
	TxCountLastHour     int     `json:"tx_count_last_hour"`
	TxCountLastDay      int     `json:"tx_count_last_day"`
	TxAmountLastHour    float64 `json:"tx_amount_last_hour"`
	TxAmountLastDay     float64 `json:"tx_amount_last_day"`

	// Geographic features
	IsKnownLocation     float64 `json:"is_known_location"` // 0 or 1
	IsCrossBorder       float64 `json:"is_cross_border"`   // 0 or 1
	IsBlockedCountry    float64 `json:"is_blocked_country"`// 0 or 1
	DistanceFromLast    float64 `json:"distance_from_last"`// km

	// Device features
	IsKnownDevice       float64 `json:"is_known_device"`   // 0 or 1
	IsTrustedDevice     float64 `json:"is_trusted_device"` // 0 or 1
	DeviceCount         int     `json:"device_count"`

	// User features
	AccountAgeDays      float64 `json:"account_age_days"`
	DaysSinceLastActivity float64 `json:"days_since_last_activity"`
	AvgTransactionAmount float64 `json:"avg_transaction_amount"`
	AmountDeviation     float64 `json:"amount_deviation"` // How many std devs from mean

	// Merchant features
	IsHighRiskMerchant  float64 `json:"is_high_risk_merchant"` // 0 or 1
	IsKnownMerchant     float64 `json:"is_known_merchant"`     // 0 or 1
}

// FeatureExtractor extracts features for ML prediction
type FeatureExtractor struct {
	highValueThreshold decimal.Decimal
	blockedCountries   map[string]bool
}

// NewFeatureExtractor creates a new feature extractor
func NewFeatureExtractor(highValueThreshold decimal.Decimal, blockedCountries []string) *FeatureExtractor {
	blocked := make(map[string]bool)
	for _, c := range blockedCountries {
		blocked[c] = true
	}

	return &FeatureExtractor{
		highValueThreshold: highValueThreshold,
		blockedCountries:   blocked,
	}
}

// Extract extracts features from a rule evaluation context
func (e *FeatureExtractor) Extract(ctx context.Context, evalCtx *fraud.RuleEvaluationContext) *Features {
	f := &Features{}

	// Transaction features
	f.Amount = evalCtx.Amount.InexactFloat64()
	f.AmountLog = logAmount(f.Amount)
	if evalCtx.Amount.GreaterThan(e.highValueThreshold) {
		f.IsHighValue = 1.0
	}
	f.Currency = evalCtx.Currency

	// Time features
	f.HourOfDay = evalCtx.Timestamp.Hour()
	f.DayOfWeek = int(evalCtx.Timestamp.Weekday())
	if f.DayOfWeek == 0 || f.DayOfWeek == 6 {
		f.IsWeekend = 1.0
	}
	if f.HourOfDay >= 22 || f.HourOfDay <= 5 {
		f.IsNightTime = 1.0
	}

	// Velocity features from recent transactions
	hourAgo := evalCtx.Timestamp.Add(-1 * time.Hour)
	dayAgo := evalCtx.Timestamp.Add(-24 * time.Hour)

	var txCountHour, txCountDay int
	var txAmountHour, txAmountDay decimal.Decimal

	for _, tx := range evalCtx.RecentTransactions {
		if tx.Timestamp.After(hourAgo) {
			txCountHour++
			txAmountHour = txAmountHour.Add(tx.Amount)
		}
		if tx.Timestamp.After(dayAgo) {
			txCountDay++
			txAmountDay = txAmountDay.Add(tx.Amount)
		}
	}

	f.TxCountLastHour = txCountHour
	f.TxCountLastDay = txCountDay
	f.TxAmountLastHour = txAmountHour.InexactFloat64()
	f.TxAmountLastDay = txAmountDay.InexactFloat64()

	// Geographic features
	if evalCtx.Location != nil {
		if e.blockedCountries[evalCtx.Location.Country] {
			f.IsBlockedCountry = 1.0
		}

		// Check if location matches payment issuing country
		if evalCtx.Payment != nil && evalCtx.Location.Country != evalCtx.Payment.IssuingCountry {
			f.IsCrossBorder = 1.0
		}

		// Distance from last transaction
		if len(evalCtx.RecentTransactions) > 0 && evalCtx.RecentTransactions[0].Location != nil {
			lastLoc := evalCtx.RecentTransactions[0].Location
			f.DistanceFromLast = haversine(
				evalCtx.Location.Latitude, evalCtx.Location.Longitude,
				lastLoc.Latitude, lastLoc.Longitude,
			)
		}
	}

	// Device features
	if evalCtx.Device != nil {
		if evalCtx.Device.IsTrustedDevice {
			f.IsTrustedDevice = 1.0
			f.IsKnownDevice = 1.0
		}

		// Check device history
		for _, d := range evalCtx.DeviceHistory {
			if d.DeviceID == evalCtx.Device.DeviceID {
				f.IsKnownDevice = 1.0
				break
			}
		}

		f.DeviceCount = len(evalCtx.DeviceHistory)
	}

	// User features
	if evalCtx.UserProfile != nil {
		f.AccountAgeDays = evalCtx.UserProfile.AccountAge.Hours() / 24
		f.DaysSinceLastActivity = time.Since(evalCtx.UserProfile.LastActivityAt).Hours() / 24
		f.AvgTransactionAmount = evalCtx.UserProfile.AverageTransaction.InexactFloat64()

		// Calculate deviation from average
		if f.AvgTransactionAmount > 0 {
			f.AmountDeviation = (f.Amount - f.AvgTransactionAmount) / f.AvgTransactionAmount
		}

		// Check if merchant is known
		if evalCtx.Merchant != nil {
			for _, m := range evalCtx.UserProfile.TypicalMerchants {
				if m == evalCtx.Merchant.MerchantID {
					f.IsKnownMerchant = 1.0
					break
				}
			}
		}

		// Check if location is typical
		if evalCtx.Location != nil {
			for _, loc := range evalCtx.UserProfile.TypicalLocations {
				if loc == evalCtx.Location.Country {
					f.IsKnownLocation = 1.0
					break
				}
			}
		}
	}

	// Merchant features
	if evalCtx.Merchant != nil && evalCtx.Merchant.IsHighRisk {
		f.IsHighRiskMerchant = 1.0
	}

	return f
}

// ToVector converts features to a float slice for ML model input
func (f *Features) ToVector() []float64 {
	return []float64{
		f.Amount,
		f.AmountLog,
		f.IsHighValue,
		float64(f.HourOfDay),
		float64(f.DayOfWeek),
		f.IsWeekend,
		f.IsNightTime,
		float64(f.TxCountLastHour),
		float64(f.TxCountLastDay),
		f.TxAmountLastHour,
		f.TxAmountLastDay,
		f.IsKnownLocation,
		f.IsCrossBorder,
		f.IsBlockedCountry,
		f.DistanceFromLast,
		f.IsKnownDevice,
		f.IsTrustedDevice,
		float64(f.DeviceCount),
		f.AccountAgeDays,
		f.DaysSinceLastActivity,
		f.AvgTransactionAmount,
		f.AmountDeviation,
		f.IsHighRiskMerchant,
		f.IsKnownMerchant,
	}
}

func logAmount(amount float64) float64 {
	if amount <= 0 {
		return 0
	}
	return math.Log10(amount + 1)
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
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

