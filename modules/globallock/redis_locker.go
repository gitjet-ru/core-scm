// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gitjet-ru/core-scm/modules/nosql"
	"github.com/redis/go-redis/v9"
)

const redisLockKeyPrefix = "gitea:globallock:"

// redisLockExpiry is the default expiry time for a lock.
// Define it as a variable to make it possible to change it in tests.
var redisLockExpiry = 30 * time.Second

type redisLocker struct {
	redis redis.UniversalClient

	mutexM   sync.Map
	closed   atomic.Bool
	extendWg sync.WaitGroup
}

var _ Locker = &redisLocker{}

func NewRedisLocker(connection string) Locker {
	l := &redisLocker{
		redis: nosql.GetManager().GetRedisClient(connection),
	}

	l.extendWg.Add(1)
	l.startExtend()

	return l
}

func (l *redisLocker) Lock(ctx context.Context, key string) (ReleaseFunc, error) {
	return l.lock(ctx, key, 0)
}

func (l *redisLocker) TryLock(ctx context.Context, key string) (bool, ReleaseFunc, error) {
	f, err := l.lock(ctx, key, 1)
	if errors.Is(err, errLockTaken) {
		return false, f, nil
	}
	return err == nil, f, err
}

// Close closes the locker.
// It will stop extending the locks and refuse to acquire new locks.
// In actual use, it is not necessary to call this function.
// But it's useful in tests to release resources.
// It could take some time since it waits for the extending goroutine to finish.
func (l *redisLocker) Close() error {
	l.closed.Store(true)
	l.extendWg.Wait()
	return nil
}

func (l *redisLocker) lock(ctx context.Context, key string, tries int) (ReleaseFunc, error) {
	if l.closed.Load() {
		return func() {}, errors.New("locker is closed")
	}

	m := &redisMutex{
		key:   redisLockKeyPrefix + key,
		value: randomLockValue(),
	}

	tryOnce := tries > 0
	for {
		ok, err := l.redis.SetNX(ctx, m.key, m.value, redisLockExpiry).Result()
		if err != nil {
			return func() {}, err
		}
		if ok {
			m.until = time.Now().Add(redisLockExpiry)
			l.mutexM.Store(key, m)

			releaseOnce := sync.Once{}
			return func() {
				releaseOnce.Do(func() {
					l.mutexM.Delete(key)
					_ = l.unlock(context.Background(), m)
				})
			}, nil
		}
		if tryOnce {
			return func() {}, errLockTaken
		}

		select {
		case <-ctx.Done():
			return func() {}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (l *redisLocker) startExtend() {
	if l.closed.Load() {
		l.extendWg.Done()
		return
	}

	toExtend := make([]*redisMutex, 0)
	l.mutexM.Range(func(_, value any) bool {
		m := value.(*redisMutex)

		// Extend the lock if it is not expired.
		// Although the mutex will be removed from the map before it is released,
		// it still can be expired because of a failed extension.
		// If it happens, it does not need to be extended anymore.
		if time.Now().After(m.until) {
			return true
		}

		toExtend = append(toExtend, m)
		return true
	})
	for _, v := range toExtend {
		_ = l.extend(context.Background(), v)
	}

	time.AfterFunc(redisLockExpiry/2, l.startExtend)
}

var errLockTaken = errors.New("lock is already held")

type redisMutex struct {
	key   string
	value string
	until time.Time
}

const unlockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

const extendScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`

func (l *redisLocker) unlock(ctx context.Context, m *redisMutex) error {
	_, err := l.redis.Eval(ctx, unlockScript, []string{m.key}, m.value).Result()
	return err
}

func (l *redisLocker) extend(ctx context.Context, m *redisMutex) error {
	ms := redisLockExpiry.Milliseconds()
	res, err := l.redis.Eval(ctx, extendScript, []string{m.key}, m.value, ms).Int64()
	if err != nil {
		return err
	}
	if res == 1 {
		m.until = time.Now().Add(redisLockExpiry)
	}
	return nil
}

func randomLockValue() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format(time.RFC3339Nano)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
