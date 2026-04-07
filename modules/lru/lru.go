// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package lru provides a small in-process LRU cache and a TwoQueue adapter
// used by the application cache layer. Implemented to avoid MPL-licensed
// third-party LRU implementations.
package lru

import (
	"container/list"
	"errors"
)

// Cache is a fixed-capacity LRU map.
type Cache[K comparable, V any] struct {
	max   int
	ll    *list.List
	items map[K]*list.Element
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

// New creates an LRU cache with the given maximum size.
func New[K comparable, V any](max int) (*Cache[K, V], error) {
	if max <= 0 {
		return nil, errors.New("invalid size")
	}
	return &Cache[K, V]{
		max:   max,
		ll:    list.New(),
		items: make(map[K]*list.Element),
	}, nil
}

// Purge clears the cache.
func (c *Cache[K, V]) Purge() {
	c.ll.Init()
	clear(c.items)
}

// Len returns the number of entries.
func (c *Cache[K, V]) Len() int {
	return c.ll.Len()
}

// Contains reports whether key is present without changing recency.
func (c *Cache[K, V]) Contains(key K) bool {
	_, ok := c.items[key]
	return ok
}

// Peek returns the value without updating recency.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	var zero V
	if el, ok := c.items[key]; ok {
		return el.Value.(*entry[K, V]).value, true
	}
	return zero, false
}

// Get returns the value and marks the entry as recently used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	var zero V
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*entry[K, V]).value, true
	}
	return zero, false
}

// Add inserts or updates a key. It may evict the oldest entry.
func (c *Cache[K, V]) Add(key K, value V) {
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*entry[K, V]).value = value
		return
	}
	if c.ll.Len() >= c.max {
		c.removeOldest()
	}
	ent := &entry[K, V]{key: key, value: value}
	el := c.ll.PushFront(ent)
	c.items[key] = el
}

// Remove removes a key if present.
func (c *Cache[K, V]) Remove(key K) bool {
	if el, ok := c.items[key]; ok {
		c.removeElement(el)
		return true
	}
	return false
}

// RemoveOldest removes the least-recently-used entry.
func (c *Cache[K, V]) RemoveOldest() (K, V, bool) {
	if c.ll.Len() == 0 {
		var k K
		var v V
		return k, v, false
	}
	el := c.ll.Back()
	ent := el.Value.(*entry[K, V])
	c.removeElement(el)
	return ent.key, ent.value, true
}

// Keys returns keys from most- to least-recently used.
func (c *Cache[K, V]) Keys() []K {
	keys := make([]K, 0, c.ll.Len())
	for el := c.ll.Front(); el != nil; el = el.Next() {
		keys = append(keys, el.Value.(*entry[K, V]).key)
	}
	return keys
}

// Values returns values from most- to least-recently used.
func (c *Cache[K, V]) Values() []V {
	vals := make([]V, 0, c.ll.Len())
	for el := c.ll.Front(); el != nil; el = el.Next() {
		vals = append(vals, el.Value.(*entry[K, V]).value)
	}
	return vals
}

// Resize changes capacity; may evict entries down to the new size.
func (c *Cache[K, V]) Resize(max int) int {
	if max <= 0 {
		return 0
	}
	c.max = max
	evicted := 0
	for c.ll.Len() > c.max {
		c.removeOldest()
		evicted++
	}
	return evicted
}

func (c *Cache[K, V]) removeOldest() {
	el := c.ll.Back()
	if el != nil {
		c.removeElement(el)
	}
}

func (c *Cache[K, V]) removeElement(el *list.Element) {
	c.ll.Remove(el)
	ent := el.Value.(*entry[K, V])
	delete(c.items, ent.key)
}
