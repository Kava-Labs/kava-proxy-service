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

	t.Run("returns a PruningOrDefaultProxies when sharding enabled", func(t *testing.T) {
		config := newConfig(t, dummyConfig.ProxyBackendHostURLMapRaw, dummyConfig.ProxyPruningBackendHostURLMapRaw)
		proxies := service.NewProxies(config, dummyLogger)
		require.IsType(t, service.PruningOrDefaultProxies{}, proxies)
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
		proxy, metadata, found := proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		require.Equal(t, metadata.BackendName, service.ResponseBackendDefault)
		requireProxyRoutesToUrl(t, proxy, req, "magicalbackend.kava.io/")

		req = mockReqForUrl("https://archive.kava.io")
		proxy, metadata, found = proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		require.Equal(t, metadata.BackendName, service.ResponseBackendDefault)
		requireProxyRoutesToUrl(t, proxy, req, "archivenode.kava.io/")

		req = mockReqForUrl("//pruning.kava.io/some/nested/endpoint")
		proxy, metadata, found = proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		require.Equal(t, metadata.BackendName, service.ResponseBackendDefault)
		requireProxyRoutesToUrl(t, proxy, req, "pruningnode.kava.io/some/nested/endpoint")
	})

	t.Run("ProxyForHost fails with unknown host", func(t *testing.T) {
		_, _, found := proxies.ProxyForRequest(mockReqForUrl("//unknown-host.kava.io"))
		require.False(t, found, "expected proxy not found for unknown host")
	})
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
