package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Load reads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()

	// Set defaults from DefaultConfig
	setDefaults(v, cfg)

	// Read from config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			// Config file not found is ok - we use defaults and env vars
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	// Read from environment variables
	v.SetEnvPrefix("FRAUD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal into config struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// LoadFromEnv loads configuration primarily from environment variables
func LoadFromEnv() (*Config, error) {
	cfg := DefaultConfig()

	// Override with environment variables
	if host := os.Getenv("FRAUD_SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if port := os.Getenv("FRAUD_SERVER_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Server.Port)
	}

	// Database
	if host := os.Getenv("FRAUD_DB_HOST"); host != "" {
		cfg.Database.Host = host
	}
	if port := os.Getenv("FRAUD_DB_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Database.Port)
	}
	if user := os.Getenv("FRAUD_DB_USER"); user != "" {
		cfg.Database.User = user
	}
	if pass := os.Getenv("FRAUD_DB_PASSWORD"); pass != "" {
		cfg.Database.Password = pass
	}
	if name := os.Getenv("FRAUD_DB_NAME"); name != "" {
		cfg.Database.Name = name
	}

	// Redis
	if host := os.Getenv("FRAUD_REDIS_HOST"); host != "" {
		cfg.Redis.Host = host
	}
	if port := os.Getenv("FRAUD_REDIS_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Redis.Port)
	}
	if pass := os.Getenv("FRAUD_REDIS_PASSWORD"); pass != "" {
		cfg.Redis.Password = pass
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper, cfg *Config) {
	// Server defaults
	v.SetDefault("server.host", cfg.Server.Host)
	v.SetDefault("server.port", cfg.Server.Port)
	v.SetDefault("server.read_timeout", cfg.Server.ReadTimeout)
	v.SetDefault("server.write_timeout", cfg.Server.WriteTimeout)

	// Database defaults
	v.SetDefault("database.host", cfg.Database.Host)
	v.SetDefault("database.port", cfg.Database.Port)
	v.SetDefault("database.user", cfg.Database.User)
	v.SetDefault("database.name", cfg.Database.Name)
	v.SetDefault("database.ssl_mode", cfg.Database.SSLMode)

	// Redis defaults
	v.SetDefault("redis.host", cfg.Redis.Host)
	v.SetDefault("redis.port", cfg.Redis.Port)
	v.SetDefault("redis.db", cfg.Redis.DB)
	v.SetDefault("redis.pool_size", cfg.Redis.PoolSize)

	// Fraud defaults
	v.SetDefault("fraud.block_threshold", cfg.Fraud.BlockThreshold)
	v.SetDefault("fraud.review_threshold", cfg.Fraud.ReviewThreshold)
	v.SetDefault("fraud.challenge_threshold", cfg.Fraud.ChallengeThreshold)
}

