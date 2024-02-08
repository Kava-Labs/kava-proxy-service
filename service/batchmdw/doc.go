// Package batchmdw is responsible for the middleware used to handle batch requests.

// The primary export is CreateBatchProcessingMiddleware which separates each individual request
// in the batch and proxies it as if it were a single request.
// The responses are then combined into a single JSON array before being sent to the client.

// A best effort is made to forward the appropriate response headers.
// The headers from the response for the first request are used with the exception of:
// - Content-Length, which is dropped to ensure client reads whole response
// - The cache status header set by cachemdw, which is updated to reflect the cache-hit status of _all_ requests.

// The cache status header will be set to:
//   - `HIT` when all requests are cache hits
//   - `MISS` when all requests are cache misses
//   - `PARTIAL` when there is a mix of cache hits and misses
package batchmdw
