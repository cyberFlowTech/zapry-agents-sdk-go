package agentsdk

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisMemoryStoreOptions configures RedisMemoryStore.
type RedisMemoryStoreOptions struct {
	Addr             string
	Username         string
	Password         string
	DB               int
	KeyPrefix        string
	OperationTimeout time.Duration
}

// DefaultRedisMemoryStoreOptions returns production-friendly defaults.
func DefaultRedisMemoryStoreOptions() RedisMemoryStoreOptions {
	return RedisMemoryStoreOptions{
		Addr:             "127.0.0.1:6379",
		KeyPrefix:        "agentsdk:memory",
		OperationTimeout: 5 * time.Second,
	}
}

// RedisMemoryStore implements MemoryStore on top of Redis.
type RedisMemoryStore struct {
	client           redis.UniversalClient
	keyPrefix        string
	operationTimeout time.Duration
}

// NewRedisMemoryStore creates a Redis-backed memory store and validates connectivity.
func NewRedisMemoryStore(opts RedisMemoryStoreOptions) (*RedisMemoryStore, error) {
	defaults := DefaultRedisMemoryStoreOptions()
	if strings.TrimSpace(opts.Addr) == "" {
		opts.Addr = defaults.Addr
	}
	if strings.TrimSpace(opts.KeyPrefix) == "" {
		opts.KeyPrefix = defaults.KeyPrefix
	}
	if opts.OperationTimeout <= 0 {
		opts.OperationTimeout = defaults.OperationTimeout
	}

	client := redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Username: opts.Username,
		Password: opts.Password,
		DB:       opts.DB,
	})

	store := &RedisMemoryStore{
		client:           client,
		keyPrefix:        opts.KeyPrefix,
		operationTimeout: opts.OperationTimeout,
	}
	if err := store.ping(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return store, nil
}

// NewRedisMemoryStoreWithClient creates a store from an existing redis client.
func NewRedisMemoryStoreWithClient(client redis.UniversalClient, keyPrefix string, operationTimeout time.Duration) (*RedisMemoryStore, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	if strings.TrimSpace(keyPrefix) == "" {
		keyPrefix = DefaultRedisMemoryStoreOptions().KeyPrefix
	}
	if operationTimeout <= 0 {
		operationTimeout = DefaultRedisMemoryStoreOptions().OperationTimeout
	}
	store := &RedisMemoryStore{
		client:           client,
		keyPrefix:        keyPrefix,
		operationTimeout: operationTimeout,
	}
	if err := store.ping(); err != nil {
		return nil, err
	}
	return store, nil
}

// Close closes the underlying redis client.
func (s *RedisMemoryStore) Close() error {
	return s.client.Close()
}

func (s *RedisMemoryStore) Get(namespace, key string) (string, error) {
	ctx, cancel := s.newContext()
	defer cancel()
	value, err := s.client.Get(ctx, s.fullKey(namespace, key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

func (s *RedisMemoryStore) Set(namespace, key, value string) error {
	ctx, cancel := s.newContext()
	defer cancel()
	return s.client.Set(ctx, s.fullKey(namespace, key), value, 0).Err()
}

func (s *RedisMemoryStore) Delete(namespace, key string) error {
	ctx, cancel := s.newContext()
	defer cancel()
	return s.client.Del(ctx, s.fullKey(namespace, key)).Err()
}

func (s *RedisMemoryStore) ListKeys(namespace string) ([]string, error) {
	ctx, cancel := s.newContext()
	defer cancel()

	pattern := s.fullKey(namespace, "*")
	namespacePrefix := s.namespacePrefix(namespace)
	seen := make(map[string]struct{})
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			trimmed := strings.TrimPrefix(key, namespacePrefix)
			if trimmed != "" {
				seen[trimmed] = struct{}{}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result, nil
}

func (s *RedisMemoryStore) Append(namespace, key, value string) error {
	ctx, cancel := s.newContext()
	defer cancel()
	return s.client.RPush(ctx, s.fullKey(namespace, key), value).Err()
}

func (s *RedisMemoryStore) GetList(namespace, key string, limit, offset int) ([]string, error) {
	if offset < 0 {
		offset = 0
	}
	start := int64(offset)
	stop := int64(-1)
	if limit > 0 {
		stop = start + int64(limit) - 1
	}
	ctx, cancel := s.newContext()
	defer cancel()
	result, err := s.client.LRange(ctx, s.fullKey(namespace, key), start, stop).Result()
	if err == redis.Nil {
		return []string{}, nil
	}
	return result, err
}

func (s *RedisMemoryStore) TrimList(namespace, key string, maxSize int) error {
	if maxSize <= 0 {
		return s.ClearList(namespace, key)
	}
	ctx, cancel := s.newContext()
	defer cancel()
	return s.client.LTrim(ctx, s.fullKey(namespace, key), -int64(maxSize), -1).Err()
}

func (s *RedisMemoryStore) ClearList(namespace, key string) error {
	ctx, cancel := s.newContext()
	defer cancel()
	return s.client.Del(ctx, s.fullKey(namespace, key)).Err()
}

func (s *RedisMemoryStore) ListLength(namespace, key string) (int, error) {
	ctx, cancel := s.newContext()
	defer cancel()
	length, err := s.client.LLen(ctx, s.fullKey(namespace, key)).Result()
	return int(length), err
}

func (s *RedisMemoryStore) fullKey(namespace, key string) string {
	namespace = strings.TrimSpace(namespace)
	key = strings.TrimSpace(key)
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, namespace, key)
}

func (s *RedisMemoryStore) namespacePrefix(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	return fmt.Sprintf("%s:%s:", s.keyPrefix, namespace)
}

func (s *RedisMemoryStore) newContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.operationTimeout)
}

func (s *RedisMemoryStore) ping() error {
	ctx, cancel := s.newContext()
	defer cancel()
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}
