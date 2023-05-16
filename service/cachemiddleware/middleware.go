package cachemiddleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/kava-labs/kava-proxy-service/decode"
)

type cacheContextKey string

const (
	// Context keys
	CachedContextKey cacheContextKey = "X-KAVA-PROXY-CACHED"

	// Headers used for caching
	CacheHitHeaderKey    = "X-Cache"
	CacheHitHeaderValue  = "HIT"
	CacheMissHeaderValue = "MISS"
)

// Middleware is a middleware that caches responses from the origin server
// and serves them from the cache if they exist.
func (c *CacheClient) Middleware(
	next http.Handler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uncachedContext := context.WithValue(r.Context(), CachedContextKey, false)

		// Skip caching if no decoded request body
		rawDecodedRequestBody := r.Context().Value(c.decodedRequestContextKey)
		decodedReq, ok := (rawDecodedRequestBody).(*decode.EVMRPCRequestEnvelope)
		if !ok {
			c.logger.Debug().Msg(fmt.Sprintf("error asserting decoded request body %s", rawDecodedRequestBody))

			next.ServeHTTP(w, r.WithContext(uncachedContext))
			return
		}

		// Check if the request is cached. This results in 3 scenarios:
		// 1. The request is cached and the response is served from the cache
		// 2. The request is not cached and we should NOT cache the response.
		//    This happens in uncacheable cases such as requesting the latest or
		//    future block, or if the request mutates chain data.
		// 3. The request is not cached and we should cache the response
		cachedBytes, found, shouldCache := c.GetCachedRequest(r.Context(), r, decodedReq)
		// 1. Serve the cached response
		if found {
			c.logger.Debug().Msg(fmt.Sprintf("found cached response for request %s", decodedReq.Method))
			w.Header().Add(CacheHitHeaderKey, CacheHitHeaderValue)
			w.Header().Add("Content-Type", http.DetectContentType(cachedBytes))
			w.Write(cachedBytes)

			// Make sure the next handler knows the response was served from cache
			// and to not hit the origin server. This does not use the uncachedContext
			cachedContext := context.WithValue(r.Context(), CachedContextKey, true)
			next.ServeHTTP(w, r.WithContext(cachedContext))
			return
		}

		// 2. This is an uncacheable request, skip any caching
		if !shouldCache {
			c.logger.Debug().Msg(fmt.Sprintf("error determining cache key %s", rawDecodedRequestBody))

			// Skip caching if we can't decode the request body
			next.ServeHTTP(w, r.WithContext(uncachedContext))
			return
		}

		// 3. Cache-able request, serve the request and cache the response
		if shouldCache {
			// Not cached, only include the cache miss header if we were able
			// to actually check the cache.
			w.Header().Add(CacheHitHeaderKey, CacheMissHeaderValue)

			// Serve the request and cache the response
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r.WithContext(uncachedContext))
			result := rec.Result()

			// Check if the response is successful
			if result.StatusCode != http.StatusOK {
				return
			}

			// Copy the response back to the original response
			body := rec.Body.Bytes()
			for k, v := range result.Header {
				w.Header().Set(k, strings.Join(v, ","))
			}
			w.WriteHeader(result.StatusCode)
			w.Write(body)

			// TODO: Determine if the response should be cached or not.
			// Requests that should not be cached include:
			// - blocks in future

			jsonMsg, err := UnmarshalJsonRpcMessage(body)
			if err != nil {
				c.logger.Debug().Err(err).Msg("error unmarshalling json rpc message")
				return
			}

			// Check if there was an error in response
			if err := jsonMsg.Error(); err != nil {
				c.logger.Debug().Err(err).Msg("response is error, not caching")
				return
			}

			// Check if the response is empty
			if jsonMsg.IsResultEmpty() {
				c.logger.Debug().Msg("response is empty, not caching")
				return
			}

			// Cache the response bytes after writing response
			if err := c.SetCachedRequest(
				context.Background(),
				r,
				decodedReq,
				body,
			); err != nil {
				c.logger.Debug().Msg(fmt.Sprintf("error caching response %s", err))
			}

			return
		}
	}
}
