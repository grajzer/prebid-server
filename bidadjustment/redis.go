package bidadjustment

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

// Client represents a Redis client
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client
func NewClient(addr, password string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,     // Redis server address, e.g., "localhost:6379"
		Password: password, // Redis password, "" if no password
		DB:       db,       // Redis database number
	})

	return &Client{
		rdb: rdb,
	}
}

// AddString adds a string value to Redis with the given key
func (c *Client) AddString(key, value string) error {
	ctx := context.Background()
	return c.rdb.Set(ctx, key, value, 0).Err()
}

// AddStringWithExpiration adds a string value to Redis with the given key and expiration time
func (c *Client) AddStringWithExpiration(key, value string, expiration time.Duration) error {
	ctx := context.Background()
	return c.rdb.Set(ctx, key, value, expiration).Err()
}

func (c *Client) GetString(key string) (string, error) {
	ctx := context.Background()
	return c.rdb.Get(ctx, key).Result()
}
