package cachemdw_test

import (
	"context"
	"math/big"
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
	defaultChainIDString = "1"
	defaultHost          = "api.kava.io"
	defaultBlockNumber   = "42"
)

var (
	defaultChainID   = big.NewInt(1)
	defaultQueryResp = []byte("resp")
)

type MockEVMBlockGetter struct{}

func NewMockEVMBlockGetter() *MockEVMBlockGetter {
	return &MockEVMBlockGetter{}
}

var _ decode.EVMBlockGetter = (*MockEVMBlockGetter)(nil)

func (c *MockEVMBlockGetter) BlockByHash(ctx context.Context, hash common.Hash) (*ethctypes.Block, error) {
	panic("not implemented")
}

func (c *MockEVMBlockGetter) ChainID(ctx context.Context) (*big.Int, error) {
	return defaultChainID, nil
}

func TestUnitTestIsCacheable(t *testing.T) {
	logger, err := logging.New("TRACE")
	require.NoError(t, err)

	blockGetter := NewMockEVMBlockGetter()
	ctxb := context.Background()

	for _, tc := range []struct {
		desc      string
		req       *decode.EVMRPCRequestEnvelope
		cacheable bool
	}{
		{
			desc:      "test case #1",
			req:       mkEVMRPCRequestEnvelope(defaultBlockNumber),
			cacheable: true,
		},
		{
			desc:      "test case #2",
			req:       mkEVMRPCRequestEnvelope("0"),
			cacheable: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			cacheable := cachemdw.IsCacheable(ctxb, blockGetter, &logger, tc.req)
			require.Equal(t, tc.cacheable, cacheable)
		})
	}
}

func TestUnitTestCacheQueryResponse(t *testing.T) {
	logger, err := logging.New("TRACE")
	require.NoError(t, err)

	inMemoryCache := cache.NewInMemoryCache()
	blockGetter := NewMockEVMBlockGetter()
	cacheTTL := time.Hour
	ctxb := context.Background()

	serviceCache := cachemdw.NewServiceCache(inMemoryCache, blockGetter, cacheTTL, service.DecodedRequestContextKey, defaultChainIDString, &logger)

	req := mkEVMRPCRequestEnvelope(defaultBlockNumber)
	resp, err := serviceCache.GetCachedQueryResponse(ctxb, req)
	require.Equal(t, cache.ErrNotFound, err)
	require.Empty(t, resp)

	err = serviceCache.CacheQueryResponse(ctxb, req, defaultChainIDString, defaultQueryResp)
	require.NoError(t, err)

	resp, err = serviceCache.GetCachedQueryResponse(ctxb, req)
	require.NoError(t, err)
	require.Equal(t, defaultQueryResp, resp)
}

func TestUnitTestValidateAndCacheQueryResponse(t *testing.T) {
	logger, err := logging.New("TRACE")
	require.NoError(t, err)

	inMemoryCache := cache.NewInMemoryCache()
	blockGetter := NewMockEVMBlockGetter()
	cacheTTL := time.Hour
	ctxb := context.Background()

	serviceCache := cachemdw.NewServiceCache(inMemoryCache, blockGetter, cacheTTL, service.DecodedRequestContextKey, defaultChainIDString, &logger)

	req := mkEVMRPCRequestEnvelope(defaultBlockNumber)
	resp, err := serviceCache.GetCachedQueryResponse(ctxb, req)
	require.Equal(t, cache.ErrNotFound, err)
	require.Empty(t, resp)

	err = serviceCache.ValidateAndCacheQueryResponse(ctxb, req, defaultQueryResp)
	require.NoError(t, err)

	resp, err = serviceCache.GetCachedQueryResponse(ctxb, req)
	require.NoError(t, err)
	require.Equal(t, defaultQueryResp, resp)
}

func mkEVMRPCRequestEnvelope(blockNumber string) *decode.EVMRPCRequestEnvelope {
	return &decode.EVMRPCRequestEnvelope{
		JSONRPCVersion: "2.0",
		ID:             1,
		Method:         "eth_getBalance",
		Params:         []interface{}{"0x1234", blockNumber},
	}
}
