package service

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// ResponseBackend values for metric reporting of where request was routed
const (
	ResponseBackendDefault = "DEFAULT"
	ResponseBackendPruning = "PRUNING"
)

// Proxies is an interface for getting a reverse proxy for a given request.
// proxy is the reverse proxy to use for the request
// responseBackend in the name of the backend that is being routed to
type Proxies interface {
	ProxyForRequest(r *http.Request) (proxy *httputil.ReverseProxy, responseBackend string, found bool)
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
	name         string
	proxyForHost map[string]*httputil.ReverseProxy
}

var _ Proxies = HostProxies{}

// ProxyForRequest implements Proxies. It determines the proxy based solely on the request Host.
func (hbp HostProxies) ProxyForRequest(r *http.Request) (*httputil.ReverseProxy, string, bool) {
	proxy, found := hbp.proxyForHost[r.Host]
	return proxy, hbp.name, found
}

// newHostProxies creates a HostProxies from the backend url map defined in the config.
func newHostProxies(name string, hostURLMap map[string]url.URL, serviceLogger *logging.ServiceLogger) HostProxies {
	reverseProxyForHost := make(map[string]*httputil.ReverseProxy)

	for host, proxyBackendURL := range hostURLMap {
		serviceLogger.Debug().Msg(fmt.Sprintf("creating reverse proxy for host %s to %+v", host, proxyBackendURL))

		targetURL := hostURLMap[host]

		reverseProxyForHost[host] = httputil.NewSingleHostReverseProxy(&targetURL)
	}

	return HostProxies{name: name, proxyForHost: reverseProxyForHost}
}

// HeightShardingProxies routes traffic based on the host _and_ the height of the query.
// If the height is "latest" (or equivalent), return Pruning node proxy host.
// Otherwise return default node proxy host.
type HeightShardingProxies struct {
	*logging.ServiceLogger

	pruningProxies HostProxies
	defaultProxies HostProxies
}

var _ Proxies = HeightShardingProxies{}

// ProxyForRequest implements Proxies.
// Decodes height of request
// - routes to Pruning proxy if defined & height is "latest"
// - otherwise routes to Default proxy
func (hsp HeightShardingProxies) ProxyForRequest(r *http.Request) (*httputil.ReverseProxy, string, bool) {
	_, _, found := hsp.pruningProxies.ProxyForRequest(r)
	// if the host isn't in the pruning proxies, short circuit fallback to default
	if !found {
		hsp.Debug().Msg(fmt.Sprintf("no pruning host backend configured for %s", r.Host))
		return hsp.defaultProxies.ProxyForRequest(r)
	}

	// parse the height of the request
	req := r.Context().Value(DecodedRequestContextKey)
	decodedReq, ok := (req).(*decode.EVMRPCRequestEnvelope)
	if !ok {
		hsp.Error().Msg("HeightShardingProxies failed to find & cast the decoded request envelope from the request context")
		return hsp.defaultProxies.ProxyForRequest(r)
	}

	// some RPC methods can always be routed to the latest block
	if decode.MethodRequiresNoHistory(decodedReq.Method) {
		hsp.Debug().Msg(fmt.Sprintf("request method %s can always use latest block. routing to pruning proxy", decodedReq.Method))
		return hsp.pruningProxies.ProxyForRequest(r)
	}

	// short circuit if requesting a method that doesn't include block height number
	if !decode.MethodHasBlockNumberParam(decodedReq.Method) {
		hsp.Debug().Msg(fmt.Sprintf("request method does not include block height (%s). routing to default proxy", decodedReq.Method))
		return hsp.defaultProxies.ProxyForRequest(r)
	}

	// parse height from the request
	height, err := decode.ParseBlockNumberFromParams(decodedReq.Method, decodedReq.Params)
	if err != nil {
		hsp.Error().Msg(fmt.Sprintf("expected but failed to parse block number for %+v: %s", decodedReq, err))
		return hsp.defaultProxies.ProxyForRequest(r)
	}

	// route "latest" to pruning proxy, otherwise route to default
	if shouldRouteToPruning(height) {
		hsp.Debug().Msg(fmt.Sprintf("request is for latest height (%d). routing to pruning proxy", height))
		return hsp.pruningProxies.ProxyForRequest(r)
	}
	hsp.Debug().Msg(fmt.Sprintf("request is for specific height (%d). routing to default proxy", height))
	return hsp.defaultProxies.ProxyForRequest(r)
}

// newHeightShardingProxies creates a new HeightShardingProxies from the service config.
func newHeightShardingProxies(config config.Config, serviceLogger *logging.ServiceLogger) HeightShardingProxies {
	return HeightShardingProxies{
		ServiceLogger:  serviceLogger,
		pruningProxies: newHostProxies(ResponseBackendPruning, config.ProxyPruningBackendHostURLMap, serviceLogger),
		defaultProxies: newHostProxies(ResponseBackendDefault, config.ProxyBackendHostURLMapParsed, serviceLogger),
	}
}

// lookup map for block tags that all represent "latest".
// maps encoded block tag -> true if the block tag should route to pruning cluster.
var blockTagEncodingsRoutedToLatest = map[int64]bool{
	decode.BlockTagToNumberCodec[decode.BlockTagLatest]:    true,
	decode.BlockTagToNumberCodec[decode.BlockTagFinalized]: true,
	decode.BlockTagToNumberCodec[decode.BlockTagPending]:   true,
	decode.BlockTagToNumberCodec[decode.BlockTagSafe]:      true,
	decode.BlockTagToNumberCodec[decode.BlockTagEmpty]:     true,
}

// shouldRouteToPruning is a helper method for determining if an encoded block tag should get routed
// to the pruning cluster
func shouldRouteToPruning(encodedHeight int64) bool {
	return blockTagEncodingsRoutedToLatest[encodedHeight]
}
