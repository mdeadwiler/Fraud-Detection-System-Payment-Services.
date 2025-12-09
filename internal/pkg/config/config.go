package config

import (
	"time"

	"github.com/shopspring/decimal"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Fraud    FraudConfig    `mapstructure:"fraud"`
	ML       MLConfig       `mapstructure:"ml"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Log      LogConfig      `mapstructure:"log"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers           []string `mapstructure:"brokers"`
	TransactionsTopic string   `mapstructure:"transactions_topic"`
	FraudAlertsTopic  string   `mapstructure:"fraud_alerts_topic"`
	ConsumerGroup     string   `mapstructure:"consumer_group"`
}

// FraudConfig holds fraud detection configuration
type FraudConfig struct {
	// Decision thresholds
	BlockThreshold     float64 `mapstructure:"block_threshold"`
	ReviewThreshold    float64 `mapstructure:"review_threshold"`
	ChallengeThreshold float64 `mapstructure:"challenge_threshold"`

	// Score weights
	VelocityWeight   float64 `mapstructure:"velocity_weight"`
	AmountWeight     float64 `mapstructure:"amount_weight"`
	GeographicWeight float64 `mapstructure:"geographic_weight"`
	DeviceWeight     float64 `mapstructure:"device_weight"`
	MerchantWeight   float64 `mapstructure:"merchant_weight"`
	BehavioralWeight float64 `mapstructure:"behavioral_weight"`
	MLWeight         float64 `mapstructure:"ml_weight"`

	// Velocity limits
	MaxTransactionsPerMinute int    `mapstructure:"max_transactions_per_minute"`
	MaxTransactionsPerHour   int    `mapstructure:"max_transactions_per_hour"`
	MaxAmountPerDay          string `mapstructure:"max_amount_per_day"` // String for YAML compatibility

	// Geographic settings
	AllowedCountries []string `mapstructure:"allowed_countries"`
	BlockedCountries []string `mapstructure:"blocked_countries"`
	MaxDistanceKm    float64  `mapstructure:"max_distance_km"`

	// High-value thresholds
	HighValueThreshold string `mapstructure:"high_value_threshold"` // String for YAML compatibility

	// Analysis timeout
	AnalysisTimeout time.Duration `mapstructure:"analysis_timeout"`
}

// GetMaxAmountPerDay returns the max amount per day as decimal
func (c *FraudConfig) GetMaxAmountPerDay() decimal.Decimal {
	d, err := decimal.NewFromString(c.MaxAmountPerDay)
	if err != nil {
		return decimal.NewFromInt(10000)
	}
	return d
}

// GetHighValueThreshold returns the high value threshold as decimal
func (c *FraudConfig) GetHighValueThreshold() decimal.Decimal {
	d, err := decimal.NewFromString(c.HighValueThreshold)
	if err != nil {
		return decimal.NewFromInt(1000)
	}
	return d
}

// MLConfig holds ML model configuration
type MLConfig struct {
	ModelPath      string        `mapstructure:"model_path"`
	ModelVersion   string        `mapstructure:"model_version"`
	FeatureCacheTTL time.Duration `mapstructure:"feature_cache_ttl"`
	Enabled        bool          `mapstructure:"enabled"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "fraud_user",
			Password:        "",
			Name:            "fraud_detection",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Redis: RedisConfig{
			Host:         "localhost",
			Port:         6379,
			Password:     "",
			DB:           0,
			PoolSize:     10,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Kafka: KafkaConfig{
			Brokers:           []string{"localhost:9092"},
			TransactionsTopic: "transactions",
			FraudAlertsTopic:  "fraud-alerts",
			ConsumerGroup:     "fraud-detection-service",
		},
		Fraud: FraudConfig{
			BlockThreshold:           0.80,
			ReviewThreshold:          0.60,
			ChallengeThreshold:       0.40,
			VelocityWeight:           0.25,
			AmountWeight:             0.15,
			GeographicWeight:         0.20,
			DeviceWeight:             0.15,
			MerchantWeight:           0.10,
			BehavioralWeight:         0.10,
			MLWeight:                 0.05,
			MaxTransactionsPerMinute: 5,
			MaxTransactionsPerHour:   30,
			MaxAmountPerDay:          "10000",
			AllowedCountries:         []string{"US", "CA", "GB", "DE", "FR"},
			BlockedCountries:         []string{},
			MaxDistanceKm:            500,
			HighValueThreshold:       "1000",
			AnalysisTimeout:          5 * time.Second,
		},
		ML: MLConfig{
			ModelPath:       "./models/fraud_model.bin",
			ModelVersion:    "v1.0.0",
			FeatureCacheTTL: 5 * time.Minute,
			Enabled:         false, // Disabled by default, rule-based works without it
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

