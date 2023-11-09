package cachemdw

import (
	"bytes"
	"io"
	"net/http"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// CachingMiddleware returns kava-proxy-service compatible middleware which works in the following way:
// - tries to get decoded request from context (previous middleware should set it)
// - checks few conditions:
//   - if request isn't already cached
//   - if request is cacheable
//   - if response is present in context
//
// - if all above is true - caches the response
// - calls next middleware
func (c *ServiceCache) CachingMiddleware(
	next http.Handler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// if cache is not enabled - do nothing and forward to next middleware
		if !c.cacheEnabled {
			c.Logger.Trace().
				Str("method", r.Method).
				Str("url", r.URL.String()).
				Str("host", r.Host).
				Msg("cache is disabled skipping caching-middleware")

			next.ServeHTTP(w, r)
			return
		}

		// if we can't get decoded request then forward to next middleware
		req := r.Context().Value(c.decodedRequestContextKey)
		decodedReq, ok := (req).(*decode.EVMRPCRequestEnvelope)
		if !ok {
			LogCannotCastRequestError(c.ServiceLogger, r)

			next.ServeHTTP(w, r)
			return
		}

		isCached := IsRequestCached(r.Context())
		cacheable := IsCacheable(c.ServiceLogger, decodedReq)
		response := r.Context().Value(ResponseContextKey)
		typedResponse, ok := response.([]byte)

		// if request isn't already cached, request is cacheable and response is present in context - cache the response
		if !isCached && cacheable && ok {
			headersToCache := getHeadersToCache(w, c.whitelistedHeaders)
			if err := c.CacheQueryResponse(
				r.Context(),
				decodedReq,
				typedResponse,
				headersToCache,
				// In this context ErrResponseIsNotCacheable isn't an actual error, it means that we can't cache the response
				// because it may change in the future.
				// For ex. it can be empty/null response for future blocks.
			); err != nil && err != ErrResponseIsNotCacheable {
				c.Logger.Error().Msgf("can't validate and cache response: %v", err)
			}
		}

		next.ServeHTTP(w, r)
	}
}

func LogCannotCastRequestError(logger *logging.ServiceLogger, r *http.Request) {
	var bodyCopy bytes.Buffer
	tee := io.TeeReader(r.Body, &bodyCopy)
	// read all body from reader into bodyBytes, and copy into bodyCopy
	bodyBytes, err := io.ReadAll(tee)
	if err != nil {
		logger.Error().Err(err).Msg("can't read lrw.body")
	}

	// replace empty body reader with fresh copy
	r.Body = io.NopCloser(&bodyCopy)

	logger.Trace().
		Str("method", r.Method).
		Str("url", r.URL.String()).
		Str("host", r.Host).
		Str("body", string(bodyBytes)).
		Msg("can't cast request to *EVMRPCRequestEnvelope type")
}

// getHeadersToCache gets header map which has to be cached along with EVM JSON-RPC response
func getHeadersToCache(w http.ResponseWriter, whitelistedHeaders []string) map[string]string {
	headersToCache := make(map[string]string, 0)

	for _, headerName := range whitelistedHeaders {
		headerValue := w.Header().Get(headerName)
		if headerValue == "" {
			continue
		}

		headersToCache[headerName] = headerValue
	}

	return headersToCache
}
