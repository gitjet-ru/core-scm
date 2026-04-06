// Copyright 2026 The GitJet Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lru

import (
	"container/list"
	"fmt"
	"sync"
)

// Keep legacy-compatible defaults used by cache config.
const (
	Default2QRecentRatio  = 0.25
	Default2QGhostEntries = 0.50
)

type entry[K comparable, V any] struct {
	key   K
	value V
}

// Cache is a simple thread-safe LRU cache.
type Cache[K comparable, V any] struct {
	mu    sync.RWMutex
	size  int
	ll    *list.List
	items map[K]*list.Element
}

func New[K comparable, V any](size int) (*Cache[K, V], error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid cache size: %d", size)
	}
	return &Cache[K, V]{
		size:  size,
		ll:    list.New(),
		items: map[K]*list.Element{},
	}, nil
}

func (c *Cache[K, V]) Add(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ele, ok := c.items[key]; ok {
		ele.Value.(*entry[K, V]).value = value
		c.ll.MoveToFront(ele)
		return
	}

	ele := c.ll.PushFront(&entry[K, V]{key: key, value: value})
	c.items[key] = ele
	if c.ll.Len() > c.size {
		c.removeOldestLocked()
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ele, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.ll.MoveToFront(ele)
	return ele.Value.(*entry[K, V]).value, true
}

func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ele, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	return ele.Value.(*entry[K, V]).value, true
}

func (c *Cache[K, V]) Remove(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ele, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeElementLocked(ele)
	return true
}

func (c *Cache[K, V]) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ll.Init()
	c.items = map[K]*list.Element{}
}

func (c *Cache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]K, 0, c.ll.Len())
	for ele := c.ll.Front(); ele != nil; ele = ele.Next() {
		keys = append(keys, ele.Value.(*entry[K, V]).key)
	}
	return keys
}

func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ll.Len()
}

func (c *Cache[K, V]) removeOldestLocked() {
	ele := c.ll.Back()
	if ele != nil {
		c.removeElementLocked(ele)
	}
}

func (c *Cache[K, V]) removeElementLocked(ele *list.Element) {
	c.ll.Remove(ele)
	kv := ele.Value.(*entry[K, V])
	delete(c.items, kv.key)
}

// TwoQueueCache is a compatibility wrapper over Cache.
// It keeps API parity where the project uses hashicorp's 2Q cache.
type TwoQueueCache[K comparable, V any] struct {
	*Cache[K, V]
}

func New2Q[K comparable, V any](size int) (*TwoQueueCache[K, V], error) {
	c, err := New[K, V](size)
	if err != nil {
		return nil, err
	}
	return &TwoQueueCache[K, V]{Cache: c}, nil
}

func New2QParams[K comparable, V any](size int, _ float64, _ float64) (*TwoQueueCache[K, V], error) {
	return New2Q[K, V](size)
}
