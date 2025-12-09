package config

import (
	"errors"
)

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return errors.New("invalid server port")
	}

	if c.Fraud.BlockThreshold < 0 || c.Fraud.BlockThreshold > 1 {
		return errors.New("block_threshold must be between 0 and 1")
	}

	if c.Fraud.ReviewThreshold < 0 || c.Fraud.ReviewThreshold > 1 {
		return errors.New("review_threshold must be between 0 and 1")
	}

	if c.Fraud.ChallengeThreshold < 0 || c.Fraud.ChallengeThreshold > 1 {
		return errors.New("challenge_threshold must be between 0 and 1")
	}

	// Thresholds should be in order: challenge < review < block
	if c.Fraud.ChallengeThreshold >= c.Fraud.ReviewThreshold {
		return errors.New("challenge_threshold should be less than review_threshold")
	}

	if c.Fraud.ReviewThreshold >= c.Fraud.BlockThreshold {
		return errors.New("review_threshold should be less than block_threshold")
	}

	return nil
}

