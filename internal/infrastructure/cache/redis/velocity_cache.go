package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// VelocityCache handles velocity tracking for fraud detection
type VelocityCache struct {
	client *Client
}

// NewVelocityCache creates a new velocity cache
func NewVelocityCache(client *Client) *VelocityCache {
	return &VelocityCache{client: client}
}

// TransactionRecord represents a cached transaction for velocity checks
type TransactionRecord struct {
	TransactionID uuid.UUID       `json:"transaction_id"`
	Amount        decimal.Decimal `json:"amount"`
	Timestamp     time.Time       `json:"timestamp"`
}

// RecordTransaction records a transaction for velocity tracking
func (c *VelocityCache) RecordTransaction(ctx context.Context, userID uuid.UUID, txID uuid.UUID, amount decimal.Decimal, timestamp time.Time) error {
	key := fmt.Sprintf("velocity:user:%s", userID.String())

	// Use sorted set with timestamp as score for efficient range queries
	member := redis.Z{
		Score:  float64(timestamp.Unix()),
		Member: fmt.Sprintf("%s|%s", txID.String(), amount.String()),
	}

	if err := c.client.ZAdd(ctx, key, member); err != nil {
		return fmt.Errorf("failed to record transaction: %w", err)
	}

	// Set expiration on the key (24 hours of data is enough for most velocity checks)
	if err := c.client.Expire(ctx, key, 24*time.Hour); err != nil {
		return fmt.Errorf("failed to set expiration: %w", err)
	}

	// Clean up old entries (older than 24 hours)
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	if err := c.client.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(cutoff, 10)); err != nil {
		// Log but don't fail - cleanup is best effort
	}

	return nil
}

// GetTransactionCount returns the number of transactions in a time window
func (c *VelocityCache) GetTransactionCount(ctx context.Context, userID uuid.UUID, window time.Duration) (int64, error) {
	key := fmt.Sprintf("velocity:user:%s", userID.String())

	minTime := time.Now().Add(-window).Unix()
	maxTime := time.Now().Unix()

	count, err := c.client.ZCount(ctx, key, strconv.FormatInt(minTime, 10), strconv.FormatInt(maxTime, 10))
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction count: %w", err)
	}

	return count, nil
}

// GetTransactionSum returns the sum of transaction amounts in a time window
func (c *VelocityCache) GetTransactionSum(ctx context.Context, userID uuid.UUID, window time.Duration) (decimal.Decimal, error) {
	key := fmt.Sprintf("velocity:user:%s", userID.String())

	minTime := time.Now().Add(-window).Unix()
	maxTime := time.Now().Unix()

	members, err := c.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: strconv.FormatInt(minTime, 10),
		Max: strconv.FormatInt(maxTime, 10),
	})
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get transactions: %w", err)
	}

	total := decimal.Zero
	for _, member := range members {
		// Parse "txID|amount" format
		var txID, amountStr string
		fmt.Sscanf(member, "%s|%s", &txID, &amountStr)

		// Find the | separator properly
		for i := len(member) - 1; i >= 0; i-- {
			if member[i] == '|' {
				amountStr = member[i+1:]
				break
			}
		}

		if amount, err := decimal.NewFromString(amountStr); err == nil {
			total = total.Add(amount)
		}
	}

	return total, nil
}

// GetRecentTransactions returns recent transactions for a user
func (c *VelocityCache) GetRecentTransactions(ctx context.Context, userID uuid.UUID, window time.Duration) ([]TransactionRecord, error) {
	key := fmt.Sprintf("velocity:user:%s", userID.String())

	minTime := time.Now().Add(-window).Unix()
	maxTime := time.Now().Unix()

	members, err := c.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: strconv.FormatInt(minTime, 10),
		Max: strconv.FormatInt(maxTime, 10),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	records := make([]TransactionRecord, 0, len(members))
	for _, member := range members {
		// Find separator
		sepIdx := -1
		for i := len(member) - 1; i >= 0; i-- {
			if member[i] == '|' {
				sepIdx = i
				break
			}
		}
		if sepIdx == -1 {
			continue
		}

		txIDStr := member[:sepIdx]
		amountStr := member[sepIdx+1:]

		txID, err := uuid.Parse(txIDStr)
		if err != nil {
			continue
		}
		amount, err := decimal.NewFromString(amountStr)
		if err != nil {
			continue
		}

		records = append(records, TransactionRecord{
			TransactionID: txID,
			Amount:        amount,
		})
	}

	return records, nil
}

// DeviceCache tracks device usage patterns
type DeviceCache struct {
	client *Client
}

// NewDeviceCache creates a new device cache
func NewDeviceCache(client *Client) *DeviceCache {
	return &DeviceCache{client: client}
}

// RecordDeviceUsage records device usage for a user
func (c *DeviceCache) RecordDeviceUsage(ctx context.Context, userID uuid.UUID, deviceID string) error {
	key := fmt.Sprintf("devices:user:%s", userID.String())

	if err := c.client.rdb.SAdd(ctx, key, deviceID).Err(); err != nil {
		return fmt.Errorf("failed to record device: %w", err)
	}

	// Set expiration (30 days of device tracking)
	if err := c.client.Expire(ctx, key, 30*24*time.Hour); err != nil {
		return fmt.Errorf("failed to set expiration: %w", err)
	}

	return nil
}

// GetDeviceCount returns the number of unique devices for a user
func (c *DeviceCache) GetDeviceCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	key := fmt.Sprintf("devices:user:%s", userID.String())
	return c.client.rdb.SCard(ctx, key).Result()
}

// IsKnownDevice checks if a device is known for a user
func (c *DeviceCache) IsKnownDevice(ctx context.Context, userID uuid.UUID, deviceID string) (bool, error) {
	key := fmt.Sprintf("devices:user:%s", userID.String())
	return c.client.rdb.SIsMember(ctx, key, deviceID).Result()
}

// LocationCache tracks user location patterns
type LocationCache struct {
	client *Client
}

// NewLocationCache creates a new location cache
func NewLocationCache(client *Client) *LocationCache {
	return &LocationCache{client: client}
}

// RecordLocation records a location for a user
func (c *LocationCache) RecordLocation(ctx context.Context, userID uuid.UUID, country, city string) error {
	key := fmt.Sprintf("locations:user:%s", userID.String())
	location := fmt.Sprintf("%s:%s", country, city)

	if err := c.client.rdb.SAdd(ctx, key, location).Err(); err != nil {
		return fmt.Errorf("failed to record location: %w", err)
	}

	if err := c.client.Expire(ctx, key, 90*24*time.Hour); err != nil {
		return fmt.Errorf("failed to set expiration: %w", err)
	}

	return nil
}

// IsKnownLocation checks if a location is known for a user
func (c *LocationCache) IsKnownLocation(ctx context.Context, userID uuid.UUID, country, city string) (bool, error) {
	key := fmt.Sprintf("locations:user:%s", userID.String())
	location := fmt.Sprintf("%s:%s", country, city)
	return c.client.rdb.SIsMember(ctx, key, location).Result()
}

// GetKnownLocations returns all known locations for a user
func (c *LocationCache) GetKnownLocations(ctx context.Context, userID uuid.UUID) ([]string, error) {
	key := fmt.Sprintf("locations:user:%s", userID.String())
	return c.client.rdb.SMembers(ctx, key).Result()
}

