package service

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// Proxies is an interface for getting a reverse proxy for a given request.
type Proxies interface {
	ProxyForRequest(r *http.Request) (proxy *httputil.ReverseProxy, found bool)
}

// NewProxies creates a Proxies instance based on the service configuration:
// - for non-sharding configuration, it returns a HostProxies
// TODO: - for sharding configurations, it returns a HeightShardingProxies
func NewProxies(config config.Config, serviceLogger *logging.ServiceLogger) Proxies {
	serviceLogger.Debug().Msg("configuring reverse proxies based solely on request host")
	return newHostProxies(config, serviceLogger)
}

// HostProxies chooses a proxy based solely on the Host of the incoming request,
// and the host -> backend url map defined in the config.
type HostProxies struct {
	proxyForHost map[string]*httputil.ReverseProxy
}

var _ Proxies = HostProxies{}

// ProxyForRequest implements Proxies. It determines the proxy based solely on the request Host.
func (hbp HostProxies) ProxyForRequest(r *http.Request) (*httputil.ReverseProxy, bool) {
	proxy, found := hbp.proxyForHost[r.Host]
	return proxy, found
}

// newHostProxies creates a HostProxies from the backend url map defined in the config.
func newHostProxies(config config.Config, serviceLogger *logging.ServiceLogger) HostProxies {
	reverseProxyForHost := make(map[string]*httputil.ReverseProxy)

	for host, proxyBackendURL := range config.ProxyBackendHostURLMapParsed {
		serviceLogger.Debug().Msg(fmt.Sprintf("creating reverse proxy for host %s to %+v", host, proxyBackendURL))

		targetURL := config.ProxyBackendHostURLMapParsed[host]

		reverseProxyForHost[host] = httputil.NewSingleHostReverseProxy(&targetURL)
	}

	return HostProxies{proxyForHost: reverseProxyForHost}
}
