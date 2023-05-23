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
	CacheHeaderKey       = "X-Cache"
	CacheHitHeaderValue  = "HIT"
	CacheMissHeaderValue = "MISS"
)

// UpdateCache updates the cache with the response from the origin server, along
// with any required data such as the chain ID.
func (c *CacheClient) UpdateCache(
	ctx context.Context,
	decodedReq *decode.EVMRPCRequestEnvelope,
	host string,
	body []byte,
) error {
	// Check the response body if we should cache it. This checks for errors
	// or empty responses.
	jsonMsg, err := UnmarshalJsonRpcMessage(body)
	if err != nil {
		return fmt.Errorf("could not unmarshal json rpc message: %w", err)
	}

	if err := jsonMsg.CheckCacheable(); err != nil {
		return fmt.Errorf("response not cacheable: %w", err)
	}

	// Get chainID from cache or origin server
	chainID, found := c.GetChainIDFromHost(ctx, host)
	if !found {
		// Fetch the chain ID for the host if it is not found in cache
		rawChainID, err := c.evmClient.ChainID(ctx)
		if err != nil {
			return fmt.Errorf("error fetching chain ID: %w", err)
		}

		// Cache the chain ID for the host
		err = c.SetChainIDForHost(ctx, host, rawChainID.String())
		if err != nil {
			return fmt.Errorf("error caching chain ID for host: %w", err)
		}

		// Update the chain ID
		chainID = rawChainID.String()
	}

	// Cache the response bytes after writing response
	if err := c.SetCachedRequest(
		ctx,
		host,
		chainID,
		decodedReq,
		body,
	); err != nil {
		return fmt.Errorf("error caching response: %w", err)
	}

	return nil
}

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
		cachedBytes, found, shouldCache := c.GetCachedRequest(r.Context(), r.Host, decodedReq)
		// 1. Serve the cached response
		if found {
			c.logger.Debug().
				Str("Method", decodedReq.Method).
				Msg("found cached response for request")

			// Add headers for cache hit and content type. This always uses
			// application/json as the content type, using http.DetectContentType
			// is unnecessary unless we want to support other content types.
			w.Header().Add(CacheHeaderKey, CacheHitHeaderValue)
			w.Header().Add("Content-Type", "application/json")
			w.Write(cachedBytes)

			// Make sure the next handler knows the response was served from
			// cache and to not hit the origin server. This continues to the
			// next handlers as there may be metrics or logging that needs to be
			// done.
			cachedContext := context.WithValue(r.Context(), CachedContextKey, true)
			next.ServeHTTP(w, r.WithContext(cachedContext))
			return
		}

		// 2. It was not found AND is an uncacheable request, skip any caching.
		if !shouldCache {
			c.logger.Debug().Msg("request is not cacheable")

			// Skip caching if we can't decode the request body
			next.ServeHTTP(w, r.WithContext(uncachedContext))
			return
		}

		// 3. Cache-able request, serve the request and cache the response

		// Not cached, only include the cache miss header if we were able
		// to actually check the cache.
		w.Header().Add(CacheHeaderKey, CacheMissHeaderValue)

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

		if err := c.UpdateCache(
			r.Context(),
			decodedReq,
			r.Host,
			body,
		); err != nil {
			c.logger.Error().Err(err).Msg("error updating cache")
		}
	}
}
