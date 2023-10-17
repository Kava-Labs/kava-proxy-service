## Caching Middleware Architecture

Package `cachemdw` is responsible for caching EVM requests and provides corresponding middleware

package can work with any underlying storage which implements simple `cache.Cache` interface

package provides two different middlewares:
- `IsCachedMiddleware` (should be run before proxy middleware)
- `CachingMiddleware`  (should be run after proxy middleware)

`IsCachedMiddleware` is responsible for setting response in the context if it's in the cache

`CachingMiddleware` is responsible for caching response by taking a value from context (should be set by `ProxyMiddleware`) and setting in the cache

## CachingMiddleware

`CachingMiddleware` returns kava-proxy-service compatible middleware which works in the following way:
- tries to get decoded request from context (previous middleware should set it)
- checks few conditions:
  - if request isn't already cached
  - if request is cacheable
  - if response is present in context
- if all above is true - caches the response
- calls next middleware

## IsCachedMiddleware

`IsCachedMiddleware` returns kava-proxy-service compatible middleware which works in the following way:
- tries to get decoded request from context (previous middleware should set it)
- tries to get response from the cache
  - if present sets cached response in context, marks as cached in context and forwards to next middleware
  - if not present marks as uncached in context and forwards to next middleware
- next middleware should check whether request was cached and act accordingly:

## What requests are cached?

As of now we cache requests which has `specific block number` in request, for example:
```json
{
	"jsonrpc":"2.0",
	"method":"eth_getBlockByNumber",
	"params":["0x1b4", true],
	"id":1
}
```

we don't cache requests without `specific block number` or requests which uses magic tags as a block number: "latest", "pending", "earliest", etc...

## Cache Invalidation

### Keys Structure

Keys have such format:

`query:<chain_name>:<method_name>:<keccak256(body)>`

For example:

`query:local-chain:eth_getBlockByNumber:0x72806e50da4f1c824b9d5a74ce9d76ac4db72e4da049802d1d6f2de3fda73e10`

### Invalidation for specific method

If you want to invalidate cache for specific method you may run such command:

`redis-cli KEYS "query:<chain_name>:<method_name>:*" | xargs redis-cli DEL`

For example:

`redis-cli KEYS "query:local-chain:eth_getBlockByNumber:*" | xargs redis-cli DEL`

### Invalidation for all methods

If you want to invalidate cache for all methods you may run such command:

`redis-cli KEYS "query:<chain_name>:*" | xargs redis-cli DEL`

For example:

`redis-cli KEYS "query:local-chain:*" | xargs redis-cli DEL`

## Architecture Diagrams

### Serve request from the cache (avoiding call to actual backend)
![image](https://github.com/Kava-Labs/kava-proxy-service/assets/37836031/1bd8cb8e-6a9e-45a6-b698-3f99eaab2aa2)

### Serve request from the backend and then cache the response
![image](https://github.com/Kava-Labs/kava-proxy-service/assets/37836031/b0eb5cb9-51da-43f9-bb7d-b94bf482f366)
