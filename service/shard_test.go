package service_test

import (
	"fmt"
	"testing"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/stretchr/testify/require"
)

func TestUnitTest_HeightShardingProxies(t *testing.T) {
	archiveBackend := "archivenode.kava.io/"
	pruningBackend := "pruningnode.kava.io/"
	config := newConfig(t,
		fmt.Sprintf("archive.kava.io>%s,pruning.kava.io>%s", archiveBackend, pruningBackend),
		fmt.Sprintf("archive.kava.io>%s", pruningBackend),
	)
	proxies := service.NewProxies(config, dummyLogger)

	testCases := []struct {
		name          string
		url           string
		req           *decode.EVMRPCRequestEnvelope
		expectFound   bool
		expectBackend string
		expectRoute   string
	}{
		// DEFAULT ROUTE CASES
		{
			name:          "routes to default when not in pruning map",
			url:           "//pruning.kava.io",
			req:           &decode.EVMRPCRequestEnvelope{},
			expectFound:   true,
			expectBackend: service.ResponseBackendDefault,
			expectRoute:   pruningBackend,
		},
		{
			name: "routes to default for specific non-latest height",
			url:  "//archive.kava.io",
			req: &decode.EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{"0xbaddad", false},
			},
			expectFound:   true,
			expectBackend: service.ResponseBackendDefault,
			expectRoute:   archiveBackend,
		},
		{
			name: "routes to default for methods that don't have block number",
			url:  "//archive.kava.io",
			req: &decode.EVMRPCRequestEnvelope{
				Method: "eth_getBlockByHash",
				Params: []interface{}{"0xe9bd10bc1d62b4406dd1fb3dbf3adb54f640bdb9ebbe3dd6dfc6bcc059275e54", false},
			},
			expectFound:   true,
			expectBackend: service.ResponseBackendDefault,
			expectRoute:   archiveBackend,
		},
		{
			name:          "routes to default if it fails to decode req",
			url:           "//archive.kava.io",
			req:           nil,
			expectFound:   true,
			expectBackend: service.ResponseBackendDefault,
			expectRoute:   archiveBackend,
		},
		{
			name: "routes to default if it fails to parse block number",
			url:  "//archive.kava.io",
			req: &decode.EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{"not-a-block-tag", false},
			},
			expectFound:   true,
			expectBackend: service.ResponseBackendDefault,
			expectRoute:   archiveBackend,
		},
		{
			name: "routes to default for 'earliest' block",
			url:  "//archive.kava.io",
			req: &decode.EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{"earliest", false},
			},
			expectFound:   true,
			expectBackend: service.ResponseBackendDefault,
			expectRoute:   archiveBackend,
		},

		// PRUNING ROUTE CASES
		{
			name: "routes to pruning for 'latest' block",
			url:  "//archive.kava.io",
			req: &decode.EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{"latest", false},
			},
			expectFound:   true,
			expectBackend: service.ResponseBackendPruning,
			expectRoute:   pruningBackend,
		},
		{
			name: "routes to pruning when block number empty",
			url:  "//archive.kava.io",
			req: &decode.EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{nil, false},
			},
			expectFound:   true,
			expectBackend: service.ResponseBackendPruning,
			expectRoute:   pruningBackend,
		},
		{
			name: "routes to pruning for no-history methods",
			url:  "//archive.kava.io",
			req: &decode.EVMRPCRequestEnvelope{
				Method: "eth_chainId",
			},
			expectFound:   true,
			expectBackend: service.ResponseBackendPruning,
			expectRoute:   pruningBackend,
		},
		{
			// this is just another example of the above, but worth pointing out!
			name: "routes to pruning when sending txs",
			url:  "//archive.kava.io",
			req: &decode.EVMRPCRequestEnvelope{
				Method: "eth_sendTransaction",
				Params: []interface{}{
					map[string]string{
						"from":     "0xdeadbeef00000000000000000000000000000123",
						"to":       "0xbaddad0000000000000000000000000000000123",
						"value":    "0x1",
						"gas":      "0xeeee",
						"gasPrice": "0x12345678900",
						"nonce":    "0x0",
					},
				},
			},
			expectFound:   true,
			expectBackend: service.ResponseBackendPruning,
			expectRoute:   pruningBackend,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := mockJsonRpcReqToUrl(tc.url, tc.req)
			proxy, responseBackend, found := proxies.ProxyForRequest(req)
			if !tc.expectFound {
				require.False(t, found, "expected proxy not to be found")
				return
			}
			require.True(t, found, "expected proxy to be found")
			require.NotNil(t, proxy)
			require.Equal(t, responseBackend, tc.expectBackend)
			requireProxyRoutesToUrl(t, proxy, req, tc.expectRoute)
		})
	}
}
