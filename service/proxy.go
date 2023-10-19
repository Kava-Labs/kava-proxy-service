package service

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// ResponseBackend values for metric reporting of where request was routed
const (
	ResponseBackendDefault = "DEFAULT"
	ResponseBackendPruning = "PRUNING"
)

// Proxies is an interface for getting a reverse proxy for a given request.
// proxy is the reverse proxy to use for the request
type Proxies interface {
	ProxyForRequest(r *http.Request) (proxy *httputil.ReverseProxy, metadata ProxyMetadata, found bool)
}

// ProxyMetadata wraps details about the proxy used for a request.
// It is useful for gathering details about the proxied request to include in metrics.
type ProxyMetadata struct {
	// name of the backend used
	BackendName string
	// url of the backend used
	BackendRoute url.URL
}

// NewProxies creates a Proxies instance based on the service configuration:
// - for non-sharding configuration, it returns a HostProxies
// - for sharding configurations, it returns a HeightShardingProxies
func NewProxies(config config.Config, serviceLogger *logging.ServiceLogger) Proxies {
	if config.EnableHeightBasedRouting {
		serviceLogger.Debug().Msg("configuring reverse proxies based on host AND height")
		return newHeightShardingProxies(config, serviceLogger)
	}
	serviceLogger.Debug().Msg("configuring reverse proxies based solely on request host")
	return newHostProxies(ResponseBackendDefault, config.ProxyBackendHostURLMapParsed, serviceLogger)
}

// HostProxies chooses a proxy based solely on the Host of the incoming request,
// and the host -> backend url map defined in the config.
// HostProxies name is the response backend provided for all requests
type HostProxies struct {
	name             string
	proxyForHost     map[string]*httputil.ReverseProxy
	targetUrlForHost map[string]url.URL
}

var _ Proxies = HostProxies{}

// ProxyForRequest implements Proxies. It determines the proxy based solely on the request Host.
func (hbp HostProxies) ProxyForRequest(r *http.Request) (*httputil.ReverseProxy, ProxyMetadata, bool) {
	proxy, found := hbp.proxyForHost[r.Host]
	metadata := ProxyMetadata{
		BackendName:  hbp.name,
		BackendRoute: hbp.targetUrlForHost[r.Host],
	}
	return proxy, metadata, found
}

// newHostProxies creates a HostProxies from the backend url map defined in the config.
func newHostProxies(name string, hostURLMap map[string]url.URL, serviceLogger *logging.ServiceLogger) HostProxies {
	reverseProxyForHost := make(map[string]*httputil.ReverseProxy)

	for host, proxyBackendURL := range hostURLMap {
		serviceLogger.Debug().Msg(fmt.Sprintf("creating reverse proxy for host %s to %+v", host, proxyBackendURL))

		targetURL := hostURLMap[host]

		reverseProxyForHost[host] = httputil.NewSingleHostReverseProxy(&targetURL)
	}

	return HostProxies{name: name, proxyForHost: reverseProxyForHost, targetUrlForHost: hostURLMap}
}
