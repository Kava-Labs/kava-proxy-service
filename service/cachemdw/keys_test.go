package cachemdw_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

func TestUnitTestBuildCacheKey(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		cachePrefix      string
		cacheItemType    cachemdw.CacheItemType
		parts            []string
		expectedCacheKey string
	}{
		{
			desc:             "test case #1",
			cachePrefix:      "chain1",
			cacheItemType:    cachemdw.CacheItemTypeEVMRequest,
			parts:            []string{"1", "2", "3"},
			expectedCacheKey: "chain1:evm-request:1:2:3",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			cacheKey := cachemdw.BuildCacheKey(tc.cachePrefix, tc.cacheItemType, tc.parts)
			require.Equal(t, tc.expectedCacheKey, cacheKey)
		})
	}
}

func TestUnitTestGetQueryKey(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		cachePrefix      string
		req              *decode.EVMRPCRequestEnvelope
		expectedCacheKey string
		errMsg           string
	}{
		{
			desc:        "test case #1",
			cachePrefix: "chain1",
			req: &decode.EVMRPCRequestEnvelope{
				JSONRPCVersion: "2.0",
				ID:             1,
				Method:         "eth_getBlockByHash",
				Params:         []interface{}{"0x1234", true},
			},
			expectedCacheKey: "chain1:evm-request:eth_getBlockByHash:sha256:2db366278f2cb463f92147bd888bdcad528b44baa94b7920fdff35f4c11ee617",
		},
		{
			desc:        "test case #1",
			cachePrefix: "chain1",
			req:         nil,
			errMsg:      "request shouldn't be nil",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			cacheKey, err := cachemdw.GetQueryKey(tc.cachePrefix, tc.req)
			if tc.errMsg == "" {
				require.NoError(t, err)
				require.Equal(t, tc.expectedCacheKey, cacheKey)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Empty(t, cacheKey)
			}
		})
	}
}
