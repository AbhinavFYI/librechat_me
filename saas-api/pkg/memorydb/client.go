package memorydb

import (
	"context"
	"saas-api/cmd/configs"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client redis.UniversalClient
}

func NewRedisClient(ctx context.Context, config *configs.Config) (*RedisClient, error) {
	// Use UniversalClient which works with both standalone and cluster Redis
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        []string{config.MemoryDBRedisURL},
		Username:     config.MemoryDBRedisUsername,
		Password:     config.MemoryDBRedisPassword,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
		PoolSize:     10,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}

	return &RedisClient{client: client}, nil
}

func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Get retrieves a value from Redis
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Set stores a value in Redis
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Del deletes a key from Redis
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}
