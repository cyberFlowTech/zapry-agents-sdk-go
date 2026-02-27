package store

import (
	"context"
	"fmt"
	"time"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
)

// RedisClient is the minimal interface required from a Redis client.
// Compatible with go-redis/v9 Client, ClusterClient, and Ring.
type RedisClient interface {
	Get(ctx context.Context, key string) StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) StatusCmd
	Del(ctx context.Context, keys ...string) IntCmd
	Keys(ctx context.Context, pattern string) StringSliceCmd
	RPush(ctx context.Context, key string, values ...interface{}) IntCmd
	LRange(ctx context.Context, key string, start, stop int64) StringSliceCmd
	LTrim(ctx context.Context, key string, start, stop int64) StatusCmd
	LLen(ctx context.Context, key string) IntCmd
	Close() error
}

// Minimal result interfaces to avoid importing go-redis directly.
type StringCmd interface {
	Result() (string, error)
}
type StatusCmd interface {
	Err() error
}
type IntCmd interface {
	Result() (int64, error)
}
type StringSliceCmd interface {
	Result() ([]string, error)
}

// RedisMemoryStore implements agentsdk.MemoryStore using Redis.
// Keys are namespaced as "mem:{namespace}:{key}" for KV
// and "mem:{namespace}:list:{key}" for lists.
type RedisMemoryStore struct {
	client RedisClient
	prefix string
	ttl    time.Duration
	ctx    context.Context
}

// RedisStoreConfig configures the Redis store.
type RedisStoreConfig struct {
	Prefix string        // key prefix, default "mem"
	TTL    time.Duration // default TTL for KV entries, 0 = no expiry
}

// NewRedisMemoryStore creates a MemoryStore backed by Redis.
func NewRedisMemoryStore(client RedisClient, config ...RedisStoreConfig) *RedisMemoryStore {
	cfg := RedisStoreConfig{Prefix: "mem"}
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "mem"
	}
	return &RedisMemoryStore{
		client: client,
		prefix: cfg.Prefix,
		ttl:    cfg.TTL,
		ctx:    context.Background(),
	}
}

func (r *RedisMemoryStore) kvKey(namespace, key string) string {
	return fmt.Sprintf("%s:%s:%s", r.prefix, namespace, key)
}

func (r *RedisMemoryStore) listKey(namespace, key string) string {
	return fmt.Sprintf("%s:%s:list:%s", r.prefix, namespace, key)
}

func (r *RedisMemoryStore) Get(namespace, key string) (string, error) {
	val, err := r.client.Get(r.ctx, r.kvKey(namespace, key)).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

func (r *RedisMemoryStore) Set(namespace, key, value string) error {
	return r.client.Set(r.ctx, r.kvKey(namespace, key), value, r.ttl).Err()
}

func (r *RedisMemoryStore) Delete(namespace, key string) error {
	_, err := r.client.Del(r.ctx, r.kvKey(namespace, key)).Result()
	return err
}

func (r *RedisMemoryStore) ListKeys(namespace string) ([]string, error) {
	pattern := fmt.Sprintf("%s:%s:*", r.prefix, namespace)
	keys, err := r.client.Keys(r.ctx, pattern).Result()
	if err != nil {
		return nil, err
	}
	prefixLen := len(fmt.Sprintf("%s:%s:", r.prefix, namespace))
	result := make([]string, 0, len(keys))
	for _, k := range keys {
		if len(k) > prefixLen {
			result = append(result, k[prefixLen:])
		}
	}
	return result, nil
}

func (r *RedisMemoryStore) Append(namespace, key, value string) error {
	_, err := r.client.RPush(r.ctx, r.listKey(namespace, key), value).Result()
	return err
}

func (r *RedisMemoryStore) GetList(namespace, key string, limit, offset int) ([]string, error) {
	start := int64(offset)
	var stop int64
	if limit > 0 {
		stop = start + int64(limit) - 1
	} else {
		stop = -1
	}
	items, err := r.client.LRange(r.ctx, r.listKey(namespace, key), start, stop).Result()
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *RedisMemoryStore) TrimList(namespace, key string, maxSize int) error {
	lk := r.listKey(namespace, key)
	return r.client.LTrim(r.ctx, lk, int64(-maxSize), -1).Err()
}

func (r *RedisMemoryStore) ClearList(namespace, key string) error {
	_, err := r.client.Del(r.ctx, r.listKey(namespace, key)).Result()
	return err
}

func (r *RedisMemoryStore) ListLength(namespace, key string) (int, error) {
	n, err := r.client.LLen(r.ctx, r.listKey(namespace, key)).Result()
	return int(n), err
}

func (r *RedisMemoryStore) Close() error {
	return r.client.Close()
}

// Compile-time interface check.
var _ agentsdk.MemoryStore = (*RedisMemoryStore)(nil)
