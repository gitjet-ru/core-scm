# cache

cache is a middleware that aim to have a transparent interface for a lot of cache implementations.

Use use many cache adapters, including memory, file, Redis, Memcache, PostgreSQL, MySQL, Ledis and Nodb.

### Installation

	go get gitea.com/go-chi/cache

## Getting Help

```go
// Cache is the interface that operates the cache data.
type Cache interface {
	// Put puts value into cache with key and expire time.
	Put(key string, val interface{}, timeout int64) error
	// Get gets cached value by given key.
	Get(key string) interface{}
	// Delete deletes cached value by given key.
	Delete(key string) error
	// Incr increases cached int-type value by given key as a counter.
	Incr(key string) error
	// Decr decreases cached int-type value by given key as a counter.
	Decr(key string) error
	// IsExist returns true if cached value exists.
	IsExist(key string) bool
	// Flush deletes all cached data.
	Flush() error
	// StartAndGC starts GC routine based on config string settings.
	StartAndGC(opt Options) error
	// Ping tests if the cache is alive
	Ping() error
}
```

## Credits

This package is a modified version of [go-macaron/cache](https://github.com/go-macaron/cache).

## License

This project is under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for the full license text.