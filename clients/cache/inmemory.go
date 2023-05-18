package cache

import (
	"context"
	"sync"
	"time"
)

type InMemoryCache struct {
	data  map[string]cacheItem
	mutex sync.RWMutex
}

var _ Cache = (*InMemoryCache)(nil)

type cacheItem struct {
	data       []byte
	expiration time.Time
}

func NewInMemoryCache() Cache {
	return &InMemoryCache{
		data: make(map[string]cacheItem),
	}
}

func (c *InMemoryCache) Set(ctx context.Context, key string, data []byte, expiration time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	expiry := time.Now().Add(expiration)
	c.data[key] = cacheItem{
		data:       data,
		expiration: expiry,
	}

	return nil
}

func (c *InMemoryCache) Get(ctx context.Context, key string) ([]byte, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, ok := c.data[key]
	if !ok || time.Now().After(item.expiration) {
		return nil, false
	}

	return item.data, true
}

func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.data, key)
	return nil
}
