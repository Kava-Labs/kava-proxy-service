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

As of now we have 4 different groups of cacheable EVM methods:
- cacheable by block number (for ex.: `eth_getBlockByNumber`)
- cacheable by block hash (for ex.: `eth_getBlockByHash`)
- static methods (for ex.: `eth_chainId`, `net_version`)
- cacheable by tx hash (for ex.: `eth_getTransactionReceipt`)

### Cacheable by block number

Cacheable by block number means that for specific:
- method
- params
- block height (which is part of params)
response won't change over time, so we can cache it indefinitely

NOTE: we don't cache requests which uses magic tags as a block number: "latest", "pending", etc... Because for such requests answer may change over time.

Example of cacheable `eth_getBlockByNumber` method
```json
{
	"jsonrpc":"2.0",
	"method":"eth_getBlockByNumber",
	"params":[
		"0x1b4", // specific block number
		true
	],
	"id":1
}
```

### Cacheable by block hash

Cacheable by block hash means that for specific:
- method
- params
- block hash (which is part of params)
response won't change over time, so we can cache it indefinitely

So it similar to cacheable by block number, but even simplier because we don't have to deal with magic tags: "latest", "pending", etc...

### Static methods

Static methods doesn't depend on anything, so we can cache them indefinitely. 

For example:

```json
{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":67}
```

```json
{"jsonrpc":"2.0","method":"net_version","params":[],"id":67}
```

### Cacheable by tx hash

Cacheable by tx hash means that for specific:
- method
- params
- tx hash (which is part of params)
response won't change over time, so we can cache it indefinitely

`NOTE`: `eth_getTransactionByHash` has an unexpected behaviour, responses for `tx in mempool` and `tx in block` are different:

`tx in mempool` example:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "blockHash": null,
    "blockNumber": null,
    "transactionIndex": null,
    "from": "0x57852ef74abc9f0da78b49d16604bbf2d81c559e",
    "gas": "0x5208",
    ...
  }
}
```

`tx in block` example
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "blockHash": "0xcc62755636e265e1f40cc0ea757477a79a233b6a417e3a8813be2ffe6859c0aa",
    "blockNumber": "0x7e8e5e",
    "transactionIndex": "0x0",
    "from": "0x57852ef74abc9f0da78b49d16604bbf2d81c559e",
    "gas": "0x5208",
    ...
  }
}
```

we can't cache `txs which is in mempool` (because response will change after `tx will be included in block`), so in source code we check if `tx is already in block`, and only if this is the case we cache the response

example how to check if tx is in a block:
```go
func (tx *tx) IsIncludedInBlock() bool {
	return tx.BlockHash != nil &&
		tx.BlockHash != "" &&
		tx.BlockNumber != nil &&
		tx.BlockNumber != "" &&
		tx.TransactionIndex != nil &&
		tx.TransactionIndex != ""
}
```

### Where to find list of methods for every group?

It can be found in source code: https://github.com/Kava-Labs/kava-proxy-service/blob/main/decode/evm_rpc.go

Look for corresponding lists:
- CacheableByBlockNumberMethods
- CacheableByBlockHashMethods
- StaticMethods

### TTL

TTL can be specified independently for each group, for ex:
```
CACHE_METHOD_HAS_BLOCK_NUMBER_PARAM_TTL_SECONDS=600
CACHE_METHOD_HAS_BLOCK_HASH_PARAM_TTL_SECONDS=1200
CACHE_STATIC_METHOD_TTL_SECONDS=-1
```

## HTTP Headers

### Caching Headers

On top of HTTP Body we also cache whitelisted HTTP Headers, whitelisted HTTP headers can be found in `WHITELISTED_HEADERS` environment variable.

As of now it contains such Headers:

```json
{
	"name" : "WHITELISTED_HEADERS",
	"value" : "Vary,Access-Control-Expose-Headers,Access-Control-Allow-Origin,Access-Control-Allow-Methods,Access-Control-Allow-Headers,Access-Control-Allow-Credentials,Access-Control-Max-Age"
}
```

So basically we iterate over `WHITELISTED_HEADERS` and if value isn't empty we add this to cache along with `cached HTTP Response Body`.

### Access-Control-Allow-Origin Headers (Cache Hit Path)

Moreover on top of it, in cache-hit path we set value for `Access-Control-Allow-Origin` header (if it's not already set), the exact value is taken from configuration and depends on the hostname, but default is `*`.

It has to be done due to very tricky case...

## Cache Invalidation

### Keys Structure

Keys have such format:

`<cache_prefix>:evm-request:<method_name>:sha256:<sha256(body)>`

For example:

`local-chain:evm-request:eth_getBlockByHash:sha256:2db366278f2cb463f92147bd888bdcad528b44baa94b7920fdff35f4c11ee617`

### Invalidation for specific method

If you want to invalidate cache for specific method you may run such command:

`redis-cli KEYS "<cache_prefix>:evm-request:<method_name>:sha256:*" | xargs redis-cli DEL`

For example:

`redis-cli KEYS "local-chain:evm-request:eth_getBlockByNumber:sha256:*" | xargs redis-cli DEL`

### Invalidation for all methods

If you want to invalidate cache for all methods you may run such command:

`redis-cli KEYS "<cache_prefix>:evm-request:*" | xargs redis-cli DEL`

For example:

`redis-cli KEYS "local-chain:evm-request:*" | xargs redis-cli DEL`

### Invalidating all cache

If you want to invalidate all cache the best way to do it is to use this command:

`redis-cli -h <redis-endpoint> FLUSHDB`

and then to make sure that cache is empty you can run:

`redis-cli -h <redis-endpoint> KEYS "*"`

alternative approach may be using this command:

`redis-cli KEYS "*" | xargs redis-cli DEL`

but it will fail due to big number of keys, so FLUSHDB is better

### Redis endpoints (NOTE: it may change in the future):
- internal-testnet: `kava-proxy-redis-internal-testnet.ba6rtz.ng.0001.use1.cache.amazonaws.com`
- public-testnet: `kava-proxy-redis-public-testnet.ba6rtz.ng.0001.use1.cache.amazonaws.com`
- mainnet: `kava-proxy-redis-mainnet.ba6rtz.ng.0001.use1.cache.amazonaws.com`

## Architecture Diagrams

### Serve request from the cache (avoiding call to actual backend)
![image](https://github.com/Kava-Labs/kava-proxy-service/assets/37836031/1bd8cb8e-6a9e-45a6-b698-3f99eaab2aa2)

### Serve request from the backend and then cache the response
![image](https://github.com/Kava-Labs/kava-proxy-service/assets/37836031/b0eb5cb9-51da-43f9-bb7d-b94bf482f366)
