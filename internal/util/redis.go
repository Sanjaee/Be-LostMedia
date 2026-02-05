package util

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"yourapp/internal/config"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisClient(cfg *config.Config) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})

	ctx := context.Background()

	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client: rdb,
		ctx:    ctx,
	}, nil
}

// Get retrieves a value from Redis by key
func (r *RedisClient) Get(key string) (string, error) {
	val, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// Set stores a value in Redis with expiration
func (r *RedisClient) Set(key string, value interface{}, expiration time.Duration) error {
	var val string
	switch v := value.(type) {
	case string:
		val = v
	default:
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		val = string(jsonBytes)
	}

	return r.client.Set(r.ctx, key, val, expiration).Err()
}

// Delete removes a key from Redis
func (r *RedisClient) Delete(key string) error {
	return r.client.Del(r.ctx, key).Err()
}

// DeletePattern removes all keys matching a pattern
func (r *RedisClient) DeletePattern(pattern string) error {
	keys, err := r.client.Keys(r.ctx, pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return r.client.Del(r.ctx, keys...).Err()
	}
	return nil
}

// Exists checks if a key exists in Redis
func (r *RedisClient) Exists(key string) (bool, error) {
	count, err := r.client.Exists(r.ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetClient returns the underlying Redis client (for advanced usage)
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}

// ZAdd adds a member with score to a sorted set
func (r *RedisClient) ZAdd(key string, score float64, member string) error {
	return r.client.ZAdd(r.ctx, key, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

// ZRange returns members of a sorted set by rank
func (r *RedisClient) ZRange(key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(r.ctx, key, start, stop).Result()
}

// ZRevRange returns members of a sorted set by rank (descending)
func (r *RedisClient) ZRevRange(key string, start, stop int64) ([]string, error) {
	return r.client.ZRevRange(r.ctx, key, start, stop).Result()
}

// ZScore returns the score of a member in a sorted set
func (r *RedisClient) ZScore(key string, member string) (float64, error) {
	return r.client.ZScore(r.ctx, key, member).Result()
}

// ZRem removes a member from a sorted set
func (r *RedisClient) ZRem(key string, member string) error {
	return r.client.ZRem(r.ctx, key, member).Err()
}

// ZIncrBy increments the score of a member in a sorted set
func (r *RedisClient) ZIncrBy(key string, increment float64, member string) error {
	return r.client.ZIncrBy(r.ctx, key, increment, member).Err()
}
