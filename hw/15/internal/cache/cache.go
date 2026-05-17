// Package cache — in-memory кэш с TTL и singleflight (защита от thundering herd).
package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry struct {
	value  []byte
	expiry time.Time
}

// Cache — простой in-memory кэш ключ→значение с TTL.
type Cache struct {
	mu    sync.RWMutex
	items map[string]entry
	sf    singleflight.Group
	now   func() time.Time
}

// New возвращает пустой кэш.
func New() *Cache {
	return &Cache{
		items: make(map[string]entry),
		now:   time.Now,
	}
}

// Get возвращает значение по ключу. Если ключа нет или истёк TTL — ok=false.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || c.now().After(e.expiry) {
		return nil, false
	}
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, true
}

// Set кладёт значение с заданным TTL.
func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	dup := make([]byte, len(value))
	copy(dup, value)

	c.mu.Lock()
	c.items[key] = entry{
		value:  dup,
		expiry: c.now().Add(ttl),
	}
	c.mu.Unlock()
}

// GetOrLoad — read-through с защитой от thundering herd (singleflight).
func (c *Cache) GetOrLoad(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loader func(ctx context.Context) ([]byte, error),
) ([]byte, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}

	v, err, _ := c.sf.Do(key, func() (any, error) {
		if v, ok := c.Get(key); ok {
			return v, nil
		}
		loaded, err := loader(ctx)
		if err != nil {
			return nil, err
		}
		c.Set(key, loaded, ttl)
		out := make([]byte, len(loaded))
		copy(out, loaded)
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	b := v.([]byte)
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}
