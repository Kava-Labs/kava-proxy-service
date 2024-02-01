package cachemdw_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethctypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

const (
	defaultCachePrefixString = "1"
	defaultBlockNumber       = "42"
)

var (
	defaultQueryResp = []byte(testEVMQueries[TestRequestWeb3ClientVersion].ResponseBody)

	defaultConfig = cachemdw.Config{
		CacheMethodHasBlockNumberParamTTL: time.Hour,
		CacheMethodHasBlockHashParamTTL:   time.Hour,
		CacheStaticMethodTTL:              time.Hour,
		CacheMethodHasTxHashParamTTL:      time.Hour,
	}
)

type MockEVMBlockGetter struct{}

func NewMockEVMBlockGetter() *MockEVMBlockGetter {
	return &MockEVMBlockGetter{}
}

var _ decode.EVMBlockGetter = (*MockEVMBlockGetter)(nil)

func (c *MockEVMBlockGetter) HeaderByHash(ctx context.Context, hash common.Hash) (*ethctypes.Header, error) {
	panic("not implemented")
}

func TestUnitTestIsCacheable(t *testing.T) {
	logger, err := logging.New("TRACE")
	require.NoError(t, err)

	for _, tc := range []struct {
		desc      string
		req       *decode.EVMRPCRequestEnvelope
		cacheable bool
	}{
		{
			desc:      "test case #1",
			req:       mkEVMRPCRequestEnvelope(defaultBlockNumber, 1),
			cacheable: true,
		},
		{
			desc:      "test case #2",
			req:       mkEVMRPCRequestEnvelope("0", 1),
			cacheable: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			cacheable := cachemdw.IsCacheable(&logger, tc.req)
			require.Equal(t, tc.cacheable, cacheable)
		})
	}
}

func TestUnitTestCacheQueryResponse(t *testing.T) {
	logger, err := logging.New("TRACE")
	require.NoError(t, err)

	inMemoryCache := cache.NewInMemoryCache()
	blockGetter := NewMockEVMBlockGetter()
	ctxb := context.Background()

	serviceCache := cachemdw.NewServiceCache(
		inMemoryCache,
		blockGetter,
		service.DecodedRequestContextKey,
		defaultCachePrefixString,
		true,
		[]string{},
		"*",
		map[string]string{},
		&defaultConfig,
		&logger,
	)

	req := mkEVMRPCRequestEnvelope(defaultBlockNumber, 1)
	resp, err := serviceCache.GetCachedQueryResponse(ctxb, req)
	require.Equal(t, cache.ErrNotFound, err)
	require.Empty(t, resp)

	err = serviceCache.CacheQueryResponse(ctxb, req, defaultQueryResp, map[string]string{})
	require.NoError(t, err)

	resp, err = serviceCache.GetCachedQueryResponse(ctxb, req)
	require.NoError(t, err)
	require.JSONEq(t, string(defaultQueryResp), string(resp.JsonRpcResponseResult))

	// same request with different ids should return same cached response, but with correct id
	stringId := "this is a string id"
	req = mkEVMRPCRequestEnvelope(defaultBlockNumber, stringId)
	resp, err = serviceCache.GetCachedQueryResponse(ctxb, req)
	require.NoError(t, err)
	expectedRes := strings.Replace(string(defaultQueryResp), "\"id\": 1", fmt.Sprintf("\"id\": \"%s\"", stringId), 1)
	require.JSONEq(t, expectedRes, string(resp.JsonRpcResponseResult))

	boolId := true
	req = mkEVMRPCRequestEnvelope(defaultBlockNumber, boolId)
	resp, err = serviceCache.GetCachedQueryResponse(ctxb, req)
	require.NoError(t, err)
	expectedRes = strings.Replace(string(defaultQueryResp), "\"id\": 1", fmt.Sprintf("\"id\": %t", boolId), 1)
	require.JSONEq(t, expectedRes, string(resp.JsonRpcResponseResult))
}

func mkEVMRPCRequestEnvelope(blockNumber string, id interface{}) *decode.EVMRPCRequestEnvelope {
	return &decode.EVMRPCRequestEnvelope{
		JSONRPCVersion: "2.0",
		ID:             id,
		Method:         "eth_getBalance",
		Params:         []interface{}{"0x1234", blockNumber},
	}
}
