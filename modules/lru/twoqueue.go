// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lru

import (
	"errors"
	"sync"
)

const (
	// Default2QRecentRatio is kept for configuration compatibility with the
	// previous 2Q implementation.
	Default2QRecentRatio = 0.25
	// Default2QGhostEntries is kept for configuration compatibility.
	Default2QGhostEntries = 0.50
)

// TwoQueueCache is a fixed-size cache with the same external API as the
// prior implementation. It uses a single LRU list while preserving total
// capacity semantics for the application cache adapter.
type TwoQueueCache[K comparable, V any] struct {
	size        int
	recentRatio float64
	ghostRatio  float64
	inner       *Cache[K, V]
	lock        sync.RWMutex
}

// New2Q creates a TwoQueueCache with default ratio parameters.
func New2Q[K comparable, V any](size int) (*TwoQueueCache[K, V], error) {
	return New2QParams[K, V](size, Default2QRecentRatio, Default2QGhostEntries)
}

// New2QParams creates a TwoQueueCache; recentRatio and ghostRatio are
// validated but eviction uses a single LRU of capacity size (compat layer).
func New2QParams[K comparable, V any](size int, recentRatio, ghostRatio float64) (*TwoQueueCache[K, V], error) {
	if size <= 0 {
		return nil, errors.New("invalid size")
	}
	if recentRatio < 0.0 || recentRatio > 1.0 {
		return nil, errors.New("invalid recent ratio")
	}
	if ghostRatio < 0.0 || ghostRatio > 1.0 {
		return nil, errors.New("invalid ghost ratio")
	}
	inner, err := New[K, V](size)
	if err != nil {
		return nil, err
	}
	return &TwoQueueCache[K, V]{
		size:        size,
		recentRatio: recentRatio,
		ghostRatio:  ghostRatio,
		inner:       inner,
	}, nil
}

// Get returns a value and updates recency.
func (c *TwoQueueCache[K, V]) Get(key K) (value V, ok bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.inner.Get(key)
}

// Add inserts a value.
func (c *TwoQueueCache[K, V]) Add(key K, value V) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.inner.Add(key, value)
}

// Len returns the number of items.
func (c *TwoQueueCache[K, V]) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Len()
}

// Resize changes the cache size.
func (c *TwoQueueCache[K, V]) Resize(size int) (evicted int) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.inner.Resize(size)
}

// Keys returns keys (order is LRU order).
func (c *TwoQueueCache[K, V]) Keys() []K {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Keys()
}

// Values returns values in Keys order.
func (c *TwoQueueCache[K, V]) Values() []V {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Values()
}

// Remove removes a key.
func (c *TwoQueueCache[K, V]) Remove(key K) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.inner.Remove(key)
}

// Purge clears the cache.
func (c *TwoQueueCache[K, V]) Purge() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.inner.Purge()
}

// Contains reports whether the key exists without updating recency.
func (c *TwoQueueCache[K, V]) Contains(key K) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Contains(key)
}

// Peek returns a value without updating recency.
func (c *TwoQueueCache[K, V]) Peek(key K) (value V, ok bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Peek(key)
}
