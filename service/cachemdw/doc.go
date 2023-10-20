// Package cachemdw is responsible for caching EVM requests and provides corresponding middleware
// package can work with any underlying storage which implements simple cache.Cache interface
//
// package provides two different middlewares:
// - IsCachedMiddleware (should be run before proxy middleware)
// - CachingMiddleware  (should be run after proxy middleware)
//
// IsCachedMiddleware is responsible for setting response in the context if it's in the cache
// CachingMiddleware is responsible for caching response by taking a value from context (should be set by proxy mdw) and setting in the cache
package cachemdw
