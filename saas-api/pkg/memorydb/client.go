package memorydb

import (
	"context"
	"crypto/tls"
	"saas-api/cmd/configs"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	*redis.ClusterClient
}

func NewRedisClient(ctx context.Context, config *configs.Config) (*RedisClient, error) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{
			config.MemoryDBRedisURL, // Add all your cluster node addresses here
		},
		Username: config.MemoryDBRedisUsername, // If using Redis ACL
		Password: config.MemoryDBRedisPassword, // If authentication is required
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			// Uncomment if you need to skip certificate verification (not recommended for production)
			// InsecureSkipVerify: true,
		},
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}

	return &RedisClient{client}, nil
}

func (r *RedisClient) Ping(ctx context.Context) error {
	return r.ClusterClient.Ping(ctx).Err()
}
