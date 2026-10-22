package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/uday68/flarehouse/internal/model"
)

type QueryCache struct {
	client *redis.Client
}

func NewQueryCache(redisAddr string) *QueryCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	return &QueryCache{client: rdb}
}

// Ping checks if Redis is reachable
func (c *QueryCache) Ping(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return c.client.Ping(ctx).Err()
}

// HashQuery generates a deterministic SHA256 key for a normalized SQL query
func HashQuery(sql string) string {
	normalized := strings.TrimSpace(strings.ToLower(sql))
	normalized = strings.Join(strings.Fields(normalized), " ")
	hasher := sha256.New()
	hasher.Write([]byte(normalized))
	return "query_cache:" + hex.EncodeToString(hasher.Sum(nil))
}

// Get attempts to retrieve a cached QueryResponse
func (c *QueryCache) Get(ctx context.Context, key string) (*model.QueryResponse, error) {
	if c.client == nil {
		return nil, fmt.Errorf("cache disabled")
	}
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var resp model.QueryResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	resp.Cached = true
	resp.CacheKey = key
	return &resp, nil
}

// Set stores a query result in cache with the given TTL
func (c *QueryCache) Set(ctx context.Context, key string, resp *model.QueryResponse, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}
