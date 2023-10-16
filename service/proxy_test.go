package service_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/stretchr/testify/require"
)

func newConfig(t *testing.T, defaultHostMap string, pruningHostMap string) config.Config {
	parsed, err := config.ParseRawProxyBackendHostURLMap(defaultHostMap)
	require.NoError(t, err)
	result := config.Config{
		ProxyBackendHostURLMapRaw:    defaultHostMap,
		ProxyBackendHostURLMapParsed: parsed,
	}
	if pruningHostMap != "" {
		result.EnableHeightBasedRouting = true
		result.ProxyPruningBackendHostURLMapRaw = pruningHostMap
		result.ProxyPruningBackendHostURLMap, err = config.ParseRawProxyBackendHostURLMap(pruningHostMap)
		require.NoError(t, err)
	}
	return result
}

func TestUnitTest_NewProxies(t *testing.T) {
	t.Run("returns a HostProxies when sharding disabled", func(t *testing.T) {
		config := newConfig(t, dummyConfig.ProxyBackendHostURLMapRaw, "")
		proxies := service.NewProxies(config, dummyLogger)
		require.IsType(t, service.HostProxies{}, proxies)
	})

	t.Run("returns a HeightShardingProxies when sharding enabled", func(t *testing.T) {
		config := newConfig(t, dummyConfig.ProxyBackendHostURLMapRaw, dummyConfig.ProxyPruningBackendHostURLMapRaw)
		proxies := service.NewProxies(config, dummyLogger)
		require.IsType(t, service.HeightShardingProxies{}, proxies)
	})
}

func TestUnitTest_HostProxies(t *testing.T) {
	config := newConfig(t,
		"magic.kava.io>magicalbackend.kava.io,archive.kava.io>archivenode.kava.io,pruning.kava.io>pruningnode.kava.io",
		"",
	)
	proxies := service.NewProxies(config, dummyLogger)

	t.Run("ProxyForHost maps to correct proxy", func(t *testing.T) {
		req := mockReqForUrl("//magic.kava.io")
		proxy, responseBackend, found := proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		require.Equal(t, responseBackend, service.ResponseBackendDefault)
		requireProxyRoutesToUrl(t, proxy, req, "magicalbackend.kava.io/")

		req = mockReqForUrl("https://archive.kava.io")
		proxy, responseBackend, found = proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		require.Equal(t, responseBackend, service.ResponseBackendDefault)
		requireProxyRoutesToUrl(t, proxy, req, "archivenode.kava.io/")

		req = mockReqForUrl("//pruning.kava.io/some/nested/endpoint")
		proxy, responseBackend, found = proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		require.Equal(t, responseBackend, service.ResponseBackendDefault)
		requireProxyRoutesToUrl(t, proxy, req, "pruningnode.kava.io/some/nested/endpoint")
	})

	t.Run("ProxyForHost fails with unknown host", func(t *testing.T) {
		_, _, found := proxies.ProxyForRequest(mockReqForUrl("//unknown-host.kava.io"))
		require.False(t, found, "expected proxy not found for unknown host")
	})
}

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

func mockReqForUrl(reqUrl string) *http.Request {
	parsed, err := url.Parse(reqUrl)
	if err != nil {
		panic(fmt.Sprintf("unable to parse url %s: %s", reqUrl, err))
	}
	if parsed.Host == "" {
		// absolute url is required for Host to be defined.
		panic(fmt.Sprintf("test requires absolute url to determine host (prefix with '//' or 'https://'): found %s", reqUrl))
	}
	return &http.Request{Host: parsed.Host, URL: parsed}
}

func mockJsonRpcReqToUrl(url string, evmReq *decode.EVMRPCRequestEnvelope) *http.Request {
	req := mockReqForUrl(url)
	// add the request into the request context, mocking previous middleware parsing
	ctx := context.Background()
	if evmReq != nil {
		ctx = context.WithValue(ctx, service.DecodedRequestContextKey, evmReq)
	}
	return req.WithContext(ctx)
}

// requireProxyRoutesToUrl is a test helper that verifies that
// the given proxy maps the provided request to the expected proxy backend
// relies on the fact that reverse proxies are given a Director that rewrite the request's URL
func requireProxyRoutesToUrl(t *testing.T, proxy *httputil.ReverseProxy, req *http.Request, expectedRoute string) {
	proxy.Director(req)
	require.Equal(t, expectedRoute, req.URL.String())
}
