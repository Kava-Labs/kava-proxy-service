package cachemiddleware_test

import (
	"math/big"
	"testing"
	"time"

	"context"
	"net/http"
	"net/http/httptest"

	"github.com/ethereum/go-ethereum/common"
	ethctypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
)

func TestUnitTestIsBodyCacheable_Valid(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "found blockByNumber",
			body: testEVMQueries[TestResponse_EthBlockByNumber_Specific].ResponseBody,
		},
		{
			name: "positive getBalance",
			body: testEVMQueries[TestResponse_EthGetBalance_Positive].ResponseBody,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jsonMsg, err := cachemiddleware.UnmarshalJsonRpcMessage([]byte(tc.body))
			require.NoError(t, err)

			err = jsonMsg.CheckCacheable()
			require.NoError(t, err)
		})
	}
}

func TestUnitTestIsBodyCacheable_EmptyResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "null",
			body: testEVMQueries[TestResponse_EthBlockByNumber_Future].ResponseBody,
		},
		{
			name: "0x0",
			body: testEVMQueries[TestResponse_EthGetBalance_Zero].ResponseBody,
		},
		{
			name: "0x",
			body: testEVMQueries[TestResponse_EthGetCode_Empty].ResponseBody,
		},
		{
			name: "empty slice",
			body: testEVMQueries[TestResponse_EthGetAccounts_Empty].ResponseBody,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jsonMsg, err := cachemiddleware.UnmarshalJsonRpcMessage([]byte(tc.body))
			require.NoError(t, err)

			err = jsonMsg.CheckCacheable()

			require.Error(t, err)
			require.Equal(t, "empty result", err.Error())
		})
	}
}

func TestUnitTestIsBodyCacheable_InvalidBody(t *testing.T) {
	body := `this fails to unmarshal to a json-rpc message`
	_, err := cachemiddleware.UnmarshalJsonRpcMessage([]byte(body))

	require.Error(t, err)
	require.Equal(t, "invalid character 'h' in literal true (expecting 'r')", err.Error())
}

func TestUnitTestIsBodyCacheable_ErrorResponse(t *testing.T) {
	// Result: null
	body := testEVMQueries[TestResponse_EthBlockByNumber_Error].ResponseBody
	jsonMsg, err := cachemiddleware.UnmarshalJsonRpcMessage([]byte(body))
	require.NoError(t, err)

	err = jsonMsg.CheckCacheable()

	require.Error(t, err)
	require.Equal(t, "message contains error: parse error (code: -32700)", err.Error())
}

func createTestRequest(
	t *testing.T,
	url string,
	reqName testReqName,
) *http.Request {
	t.Helper()

	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	testDecodedReq, err := decode.DecodeEVMRPCRequest(
		[]byte(getTestRequestBody(reqName)),
	)
	require.NoError(t, err)

	decodedRequestBodyContext := context.WithValue(
		req.Context(),
		service.DecodedRequestContextKey,
		testDecodedReq,
	)

	req = req.WithContext(decodedRequestBodyContext)

	return req
}

func TestCacheClient_Middleware(t *testing.T) {
	logger, err := logging.New("TRACE")
	require.NoError(t, err)

	// ------------------------------------------
	// Create a new request
	req1 := createTestRequest(
		t,
		"https://api.kava.io:8545/thisshouldntshowup",
		TestResponse_EthBlockByNumber_Specific,
	)

	// Create a new cache client with in memory backend cache
	memCache := cache.NewInMemoryCache()

	cacheClient := cachemiddleware.NewClient(
		memCache,
		&TestEVMClient{},
		0, // TTL: no expiry
		service.DecodedRequestContextKey,
		&logger,
	)

	// Create a new handler that always returns a 200 OK response with the
	// corresponding response body
	resp1 := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only write the response body if the request was not cached
		if !cachemiddleware.IsRequestCached(r.Context()) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(getTestResponseBody(TestResponse_EthBlockByNumber_Specific)))
		}
	})

	// First request -- Cache Miss

	t.Run("uncached", func(t *testing.T) {
		cacheClient.Middleware(handler).ServeHTTP(resp1, req1)

		assert.Equal(t, http.StatusOK, resp1.Code)
		assert.JSONEq(t, getTestResponseBody(TestResponse_EthBlockByNumber_Specific), resp1.Body.String())
		assert.Equal(t, cachemiddleware.CacheMissHeaderValue, resp1.Header().Get(cachemiddleware.CacheHeaderKey))

		// Check if cache contains the correct keys
		cacheItems := memCache.GetAll(context.Background())
		assert.Len(t, cacheItems, 2)
		assert.Contains(t, cacheItems, "chain:api.kava.io:8545")
		assert.Contains(t, cacheItems, "query:2222:0x5236d50a560cff0174f14be10bd00a21e8d73e89a200fbd219769b6aee297131")
	})

	t.Run("cached", func(t *testing.T) {
		req2 := createTestRequest(
			t,
			"https://api.kava.io:8545/thisshouldntshowup",
			TestResponse_EthBlockByNumber_Specific,
		)

		// Second request -- Cache hit
		resp2 := httptest.NewRecorder()
		cacheClient.Middleware(handler).ServeHTTP(resp2, req2)

		assert.Equal(t, http.StatusOK, resp2.Code)
		assert.JSONEq(
			t,
			getTestResponseBody(TestResponse_EthBlockByNumber_Specific),
			resp2.Body.String(),
		)
		assert.Equal(
			t,
			cachemiddleware.CacheHitHeaderValue,
			resp2.Header().Get(cachemiddleware.CacheHeaderKey),
		)
	})
}

func TestCacheClient_Middleware_UncacheableRequest(t *testing.T) {
	ctxKey := "key"
	memCache := cache.NewInMemoryCache()

	// Create a new cache client
	cacheClient := cachemiddleware.NewClient(
		memCache,
		nil, // evmClient,
		time.Minute,
		ctxKey,
		nil, // logger
	)

	// Create a new request with a request body that cannot be decoded
	req, err := http.NewRequest("POST", "/test", nil)
	assert.NoError(t, err)
	req = req.WithContext(context.WithValue(req.Context(), ctxKey, "invalid"))

	// Create a new response recorder
	rr := httptest.NewRecorder()

	// Create a new handler that always returns a 200 OK response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Call the middleware with the handler
	cacheClient.Middleware(handler).ServeHTTP(rr, req)

	// Assert that the response is a 200 OK response and that it was not served from the cache
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test response", rr.Body.String())
	assert.Equal(t, "miss", rr.Header().Get("X-Cache"))
}

func TestCacheClient_Middleware_ErrorUpdatingCache(t *testing.T) {
	memCache := cache.NewInMemoryCache()

	// Create a new cache client with a mock cache that always returns an error
	cacheClient := cachemiddleware.NewClient(
		memCache,
		nil, // evmClient,
		time.Minute,
		"key",
		nil, // logger
	)

	// Create a new request
	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)

	// Create a new response recorder
	rr := httptest.NewRecorder()

	// Create a new handler that always returns a 200 OK response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Call the middleware with the handler
	cacheClient.Middleware(handler).ServeHTTP(rr, req)

	// Assert that the response is a 200 OK response and that it was not served from the cache
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "test response", rr.Body.String())
	assert.Equal(t, "miss", rr.Header().Get("X-Cache"))
}

type TestEVMClient struct{}

var _ cachemiddleware.EVMClient = (*TestEVMClient)(nil)

func (c *TestEVMClient) BlockByHash(ctx context.Context, hash common.Hash) (*ethctypes.Block, error) {
	panic("unimplemented")
}

func (c *TestEVMClient) ChainID(ctx context.Context) (*big.Int, error) {
	return big.NewInt(2222), nil
}
