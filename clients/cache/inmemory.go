package cache

import (
	"context"
	"sync"
	"time"
)

// InMemoryCache is an in-memory implementation of the Cache interface.
type InMemoryCache struct {
	data  map[string]cacheItem
	mutex sync.RWMutex
}

// Ensure InMemoryCache implements the Cache interface.
var _ Cache = (*InMemoryCache)(nil)

// cacheItem represents an item stored in the cache.
type cacheItem struct {
	data       []byte
	expiration time.Time
}

// NewInMemoryCache creates a new instance of InMemoryCache.
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		data: make(map[string]cacheItem),
	}
}

// Set sets the value of a key in the cache.
func (c *InMemoryCache) Set(
	ctx context.Context,
	key string,
	data []byte,
	expiration time.Duration,
	cacheIndefinitely bool,
) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	expiry := time.Now().Add(expiration)

	if cacheIndefinitely {
		// 100 years in the future to prevent expiry
		expiry = time.Now().AddDate(100, 0, 0)
	}

	c.data[key] = cacheItem{
		data:       data,
		expiration: expiry,
	}

	return nil
}

// Get retrieves the value of a key from the cache.
func (c *InMemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, ok := c.data[key]
	if !ok || time.Now().After(item.expiration) {
		// Not a real ttl but just replicates it for fetching
		delete(c.data, key)

		return nil, ErrNotFound
	}

	return item.data, nil
}

// GetAll returns all the non-expired data in the cache.
func (c *InMemoryCache) GetAll(ctx context.Context) map[string][]byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	result := make(map[string][]byte)

	for key, item := range c.data {
		if time.Now().After(item.expiration) {
			delete(c.data, key)
		} else {
			result[key] = item.data
		}
	}

	return result
}

// Delete removes a key from the cache.
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.data, key)
	return nil
}

func (c *InMemoryCache) Healthcheck(ctx context.Context) error {
	return nil
}
