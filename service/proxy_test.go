package service_test

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/stretchr/testify/require"
)

func newConfig(t *testing.T, rawHostMap string) config.Config {
	parsed, err := config.ParseRawProxyBackendHostURLMap(rawHostMap)
	require.NoError(t, err)
	return config.Config{
		ProxyBackendHostURLMapRaw:    rawHostMap,
		ProxyBackendHostURLMapParsed: parsed,
	}
}

func TestUnitTest_NewProxies(t *testing.T) {
	t.Run("returns a HostProxies when sharding disabled", func(t *testing.T) {
		config := newConfig(t, dummyConfig.ProxyBackendHostURLMapRaw)
		proxies := service.NewProxies(config, dummyLogger)
		require.IsType(t, service.HostProxies{}, proxies)
	})
	// TODO: HeightShardingProxies
}

func TestUnitTest_HostProxies(t *testing.T) {
	config := newConfig(t,
		"magic.kava.io>magicalbackend.kava.io,archive.kava.io>archivenode.kava.io,pruning.kava.io>pruningnode.kava.io",
	)
	proxies := service.NewProxies(config, dummyLogger)

	t.Run("ProxyForHost maps to correct proxy", func(t *testing.T) {
		req := mockReqForUrl("//magic.kava.io")
		proxy, found := proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		requireProxyRoutesToUrl(t, proxy, req, "magicalbackend.kava.io/")

		req = mockReqForUrl("https://archive.kava.io")
		proxy, found = proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		requireProxyRoutesToUrl(t, proxy, req, "archivenode.kava.io/")

		req = mockReqForUrl("//pruning.kava.io/some/nested/endpoint")
		proxy, found = proxies.ProxyForRequest(req)
		require.True(t, found, "expected proxy to be found")
		requireProxyRoutesToUrl(t, proxy, req, "pruningnode.kava.io/some/nested/endpoint")
	})

	t.Run("ProxyForHost fails with unknown host", func(t *testing.T) {
		_, found := proxies.ProxyForRequest(mockReqForUrl("//unknown-host.kava.io"))
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

// requireProxyRoutesToUrl is a test helper that verifies that
// the given proxy maps the provided request to the expected proxy backend
// relies on the fact that reverse proxies are given a Director that rewrite the request's URL
func requireProxyRoutesToUrl(t *testing.T, proxy *httputil.ReverseProxy, req *http.Request, expectedRoute string) {
	proxy.Director(req)
	require.Equal(t, expectedRoute, req.URL.String())
}
