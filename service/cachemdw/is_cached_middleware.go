package cachemdw

import (
	"context"
	"net/http"

	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
)

const (
	CachedContextKey   = "X-KAVA-PROXY-CACHED"
	ResponseContextKey = "X-KAVA-PROXY-RESPONSE"

	CacheHeaderKey       = "X-Kava-Proxy-Cache-Status"
	CacheHitHeaderValue  = "HIT"
	CacheMissHeaderValue = "MISS"
)

// IsCachedMiddleware returns kava-proxy-service compatible middleware which works in the following way:
// - tries to get decoded request from context (previous middleware should set it)
// - tries to get response from the cache
//   - if present sets cached response in context, marks as cached in context and forwards to next middleware
//   - if not present marks as uncached in context and forwards to next middleware
//
// - next middleware should check whether request was cached and act accordingly:
func (c *ServiceCache) IsCachedMiddleware(
	next http.Handler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// if cache is not enabled - do nothing and forward to next middleware
		if !c.cacheEnabled {
			next.ServeHTTP(w, r)
			return
		}

		uncachedContext := context.WithValue(r.Context(), CachedContextKey, false)
		cachedContext := context.WithValue(r.Context(), CachedContextKey, true)

		// if we can't get decoded request then forward to next middleware
		req := r.Context().Value(c.decodedRequestContextKey)
		decodedReq, ok := (req).(*decode.EVMRPCRequestEnvelope)
		if !ok {
			c.Logger.Error().
				Str("method", r.Method).
				Str("url", r.URL.String()).
				Str("host", r.Host).
				Msg("can't cast request to *EVMRPCRequestEnvelope type")

			next.ServeHTTP(w, r.WithContext(uncachedContext))
			return
		}

		// Check if the request is cached:
		// 1. if not cached or we encounter an error then mark as uncached and forward to next middleware
		// 2. if cached then mark as cached, set cached response in context and forward to next middleware
		cachedQueryResponse, err := c.GetCachedQueryResponse(r.Context(), decodedReq)
		if err != nil && err != cache.ErrNotFound {
			// log unexpected error
			c.Logger.Error().
				Err(err).
				Msg("error during getting response from cache")
		}
		if err != nil {
			// 1. if not cached or we encounter an error then mark as uncached and forward to next middleware
			next.ServeHTTP(w, r.WithContext(uncachedContext))
			return
		}

		// 2. if cached then mark as cached, set cached response in context and forward to next middleware
		responseContext := context.WithValue(cachedContext, ResponseContextKey, cachedQueryResponse)
		next.ServeHTTP(w, r.WithContext(responseContext))
	}
}

// IsRequestCached returns whether request was cached
// if returns true it means:
// - middleware marked that request was cached
// - value of cached response should be available in context via ResponseContextKey
func IsRequestCached(ctx context.Context) bool {
	cached, ok := ctx.Value(CachedContextKey).(bool)
	return ok && cached
}
