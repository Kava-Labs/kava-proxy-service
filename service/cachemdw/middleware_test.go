package cachemdw_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

func TestUnitTestServiceCacheMiddleware(t *testing.T) {
	logger, err := logging.New("TRACE")
	require.NoError(t, err)

	inMemoryCache := cache.NewInMemoryCache()
	blockGetter := NewMockEVMBlockGetter()
	cacheTTL := time.Duration(0)
	cacheIndefinitely := true

	serviceCache := cachemdw.NewServiceCache(
		inMemoryCache,
		blockGetter,
		cacheTTL,
		cacheIndefinitely,
		service.DecodedRequestContextKey,
		defaultCachePrefixString,
		true,
		&logger,
	)

	emptyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	cachingMdw := serviceCache.CachingMiddleware(emptyHandler)
	// proxyHandler emulates behaviour of actual service proxy handler
	// sequence of execution:
	// - isCachedMdw
	// - proxyHandler
	// - cachingMdw
	// - emptyHandler
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []byte(testEVMQueries[TestRequestEthBlockByNumberSpecific].ResponseBody)
		if cachemdw.IsRequestCached(r.Context()) {
			w.Header().Add(cachemdw.CacheHeaderKey, cachemdw.CacheHitHeaderValue)
		} else {
			w.Header().Add(cachemdw.CacheHeaderKey, cachemdw.CacheMissHeaderValue)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(response)
		responseContext := context.WithValue(r.Context(), cachemdw.ResponseContextKey, response)

		cachingMdw.ServeHTTP(w, r.WithContext(responseContext))
	})
	isCachedMdw := serviceCache.IsCachedMiddleware(proxyHandler)

	// test cache MISS and cache HIT scenarios for specified method
	// check corresponding values in cachemdw.CacheHeaderKey HTTP header

	t.Run("cache miss", func(t *testing.T) {
		req := createTestHttpRequest(
			t,
			"https://api.kava.io:8545/thisshouldntshowup",
			TestRequestEthBlockByNumberSpecific,
		)
		resp := httptest.NewRecorder()

		isCachedMdw.ServeHTTP(resp, req)

		require.Equal(t, http.StatusOK, resp.Code)
		require.JSONEq(t, testEVMQueries[TestRequestEthBlockByNumberSpecific].ResponseBody, resp.Body.String())
		require.Equal(t, cachemdw.CacheMissHeaderValue, resp.Header().Get(cachemdw.CacheHeaderKey))

		cacheItems := inMemoryCache.GetAll(context.Background())
		require.Len(t, cacheItems, 1)
		require.Contains(t, cacheItems, "1:evm-request:eth_getBlockByNumber:sha256:bf79de57723b25b85391513b470ea6989e7c44dd9afc0c270ee961c9f12f578d")
	})

	t.Run("cache hit", func(t *testing.T) {
		req := createTestHttpRequest(
			t,
			"https://api.kava.io:8545/thisshouldntshowup",
			TestRequestEthBlockByNumberSpecific,
		)
		resp := httptest.NewRecorder()

		isCachedMdw.ServeHTTP(resp, req)

		require.Equal(t, http.StatusOK, resp.Code)
		require.JSONEq(t, testEVMQueries[TestRequestEthBlockByNumberSpecific].ResponseBody, resp.Body.String())
		require.Equal(t, cachemdw.CacheHitHeaderValue, resp.Header().Get(cachemdw.CacheHeaderKey))

		cacheItems := inMemoryCache.GetAll(context.Background())
		require.Len(t, cacheItems, 1)
		require.Contains(t, cacheItems, "1:evm-request:eth_getBlockByNumber:sha256:bf79de57723b25b85391513b470ea6989e7c44dd9afc0c270ee961c9f12f578d")
	})
}

func TestUnitTestServiceCacheMiddleware_CacheIsDisabled(t *testing.T) {
	logger, err := logging.New("TRACE")
	require.NoError(t, err)

	inMemoryCache := cache.NewInMemoryCache()
	blockGetter := NewMockEVMBlockGetter()
	cacheTTL := time.Duration(0)
	cacheIndefinitely := true

	serviceCache := cachemdw.NewServiceCache(
		inMemoryCache,
		blockGetter,
		cacheTTL,
		cacheIndefinitely,
		service.DecodedRequestContextKey,
		defaultCachePrefixString,
		false,
		&logger,
	)

	emptyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	cachingMdw := serviceCache.CachingMiddleware(emptyHandler)
	// proxyHandler emulates behaviour of actual service proxy handler
	// sequence of execution:
	// - isCachedMdw
	// - proxyHandler
	// - cachingMdw
	// - emptyHandler
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []byte(testEVMQueries[TestRequestEthBlockByNumberSpecific].ResponseBody)
		if cachemdw.IsRequestCached(r.Context()) {
			w.Header().Add(cachemdw.CacheHeaderKey, cachemdw.CacheHitHeaderValue)
		} else {
			w.Header().Add(cachemdw.CacheHeaderKey, cachemdw.CacheMissHeaderValue)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(response)
		responseContext := context.WithValue(r.Context(), cachemdw.ResponseContextKey, response)

		cachingMdw.ServeHTTP(w, r.WithContext(responseContext))
	})
	isCachedMdw := serviceCache.IsCachedMiddleware(proxyHandler)

	// both calls should lead to cache MISS scenario, because cache is disabled
	// check corresponding values in cachemdw.CacheHeaderKey HTTP header

	t.Run("cache miss", func(t *testing.T) {
		req := createTestHttpRequest(
			t,
			"https://api.kava.io:8545/thisshouldntshowup",
			TestRequestEthBlockByNumberSpecific,
		)
		resp := httptest.NewRecorder()

		isCachedMdw.ServeHTTP(resp, req)

		require.Equal(t, http.StatusOK, resp.Code)
		require.JSONEq(t, testEVMQueries[TestRequestEthBlockByNumberSpecific].ResponseBody, resp.Body.String())
		require.Equal(t, cachemdw.CacheMissHeaderValue, resp.Header().Get(cachemdw.CacheHeaderKey))

		cacheItems := inMemoryCache.GetAll(context.Background())
		require.Len(t, cacheItems, 0)
	})

	t.Run("cache miss again (cache is disabled)", func(t *testing.T) {
		req := createTestHttpRequest(
			t,
			"https://api.kava.io:8545/thisshouldntshowup",
			TestRequestEthBlockByNumberSpecific,
		)
		resp := httptest.NewRecorder()

		isCachedMdw.ServeHTTP(resp, req)

		require.Equal(t, http.StatusOK, resp.Code)
		require.JSONEq(t, testEVMQueries[TestRequestEthBlockByNumberSpecific].ResponseBody, resp.Body.String())
		require.Equal(t, cachemdw.CacheMissHeaderValue, resp.Header().Get(cachemdw.CacheHeaderKey))

		cacheItems := inMemoryCache.GetAll(context.Background())
		require.Len(t, cacheItems, 0)
	})
}

func createTestHttpRequest(
	t *testing.T,
	url string,
	reqName testReqName,
) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	decodedReq, err := decode.DecodeEVMRPCRequest(
		[]byte(testEVMQueries[reqName].RequestBody),
	)
	require.NoError(t, err)

	decodedReqCtx := context.WithValue(
		req.Context(),
		service.DecodedRequestContextKey,
		decodedReq,
	)
	req = req.WithContext(decodedReqCtx)

	return req
}
