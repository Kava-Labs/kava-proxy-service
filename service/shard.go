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

// PruningOrDefaultProxies routes traffic based on the host _and_ the height of the query.
// If the height is "latest" (or equivalent), return Pruning node proxy host.
// Otherwise return default node proxy host.
type PruningOrDefaultProxies struct {
	*logging.ServiceLogger

	pruningProxies HostProxies
	defaultProxies HostProxies
}

var _ Proxies = PruningOrDefaultProxies{}

// ProxyForRequest implements Proxies.
// Decodes height of request
// - routes to Pruning proxy if defined & height is "latest"
// - otherwise routes to Default proxy
func (hsp PruningOrDefaultProxies) ProxyForRequest(r *http.Request) (*httputil.ReverseProxy, ProxyMetadata, bool) {
	_, _, found := hsp.pruningProxies.ProxyForRequest(r)
	// if the host isn't in the pruning proxies, short circuit fallback to default
	if !found {
		hsp.Trace().Msg(fmt.Sprintf("no pruning host backend configured for %s", r.Host))
		return hsp.defaultProxies.ProxyForRequest(r)
	}

	// parse the height of the request
	req := r.Context().Value(DecodedRequestContextKey)
	decodedReq, ok := (req).(*decode.EVMRPCRequestEnvelope)
	if !ok {
		hsp.Trace().Msg("PruningOrDefaultProxies failed to find & cast the decoded request envelope from the request context")
		return hsp.defaultProxies.ProxyForRequest(r)
	}

	// some RPC methods can always be routed to the latest block
	if decode.MethodRequiresNoHistory(decodedReq.Method) {
		hsp.Trace().Msg(fmt.Sprintf("request method %s can always use latest block. routing to pruning proxy", decodedReq.Method))
		return hsp.pruningProxies.ProxyForRequest(r)
	}

	// short circuit if requesting a method that doesn't include block height number
	if !decode.MethodHasBlockNumberParam(decodedReq.Method) {
		hsp.Trace().Msg(fmt.Sprintf("request method does not include block height (%s). routing to default proxy", decodedReq.Method))
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
		hsp.Trace().Msg(fmt.Sprintf("request is for latest height (%d). routing to pruning proxy", height))
		return hsp.pruningProxies.ProxyForRequest(r)
	}
	hsp.Trace().Msg(fmt.Sprintf("request is for specific height (%d). routing to default proxy", height))
	return hsp.defaultProxies.ProxyForRequest(r)
}

// newPruningOrDefaultProxies creates a new PruningOrDefaultProxies from the service config.
func newPruningOrDefaultProxies(config config.Config, serviceLogger *logging.ServiceLogger) PruningOrDefaultProxies {
	return PruningOrDefaultProxies{
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

// ShardProxies handles routing requests for specific heights to backends that contain the height.
// The height is parsed out of requests that would route to the default backend of the underlying `defaultProxies`
// If the height is contained by a backend in the host's IntervalURLMap, it is routed to that url.
// Otherwise, it forwards the request via the wrapped defaultProxies.
type ShardProxies struct {
	*logging.ServiceLogger

	defaultProxies Proxies
	shardsByHost   map[string]config.IntervalURLMap
	proxyByURL     map[*url.URL]*httputil.ReverseProxy
}

var _ Proxies = ShardProxies{}

// ProxyForRequest implements Proxies.
func (sp ShardProxies) ProxyForRequest(r *http.Request) (*httputil.ReverseProxy, ProxyMetadata, bool) {
	// short circuit if not host not in shards map
	shardsForHost, found := sp.shardsByHost[r.Host]
	if !found {
		return sp.defaultProxies.ProxyForRequest(r)
	}

	// handle unsupported hosts or routing to pruning (if enabled)
	proxy, metadata, found := sp.defaultProxies.ProxyForRequest(r)
	if metadata.BackendName != ResponseBackendDefault || !found {
		return proxy, metadata, found
	}

	// get decoded request
	req := r.Context().Value(DecodedRequestContextKey)
	decodedReq, ok := (req).(*decode.EVMRPCRequestEnvelope)
	if !ok {
		sp.Trace().Msg("PruningOrDefaultProxies failed to find & cast the decoded request envelope from the request context")
		return sp.defaultProxies.ProxyForRequest(r)
	}

	// parse height from the request
	parsedHeight, err := decode.ParseBlockNumberFromParams(decodedReq.Method, decodedReq.Params)
	if err != nil {
		sp.Error().Msg(fmt.Sprintf("expected but failed to parse block number for %+v: %s", decodedReq, err))
		return sp.defaultProxies.ProxyForRequest(r)
	}

	// handle encoded block numbers
	height := parsedHeight
	if height == decode.BlockTagToNumberCodec[decode.BlockTagEarliest] {
		// convert "earliest" to "1" so it routes to first shard
		height = 1
	} else if parsedHeight < 1 {
		// route all other encoded tags to default proxy.
		// in practice, this is unreachable because they will be handled by the pruning Proxies
		// if shard routing is enabled without PruningOrDefaultProxies, this handles all special block tags
		return sp.defaultProxies.ProxyForRequest(r)
	}

	// look for shard including height
	url, shardHeight, found := shardsForHost.Lookup(uint64(height))
	if !found {
		return sp.defaultProxies.ProxyForRequest(r)
	}

	// shard exists, route to it!
	metadata = ProxyMetadata{
		BackendName:    ResponseBackendShard,
		BackendRoute:   *url,
		ShardEndHeight: shardHeight,
	}
	return sp.proxyByURL[url], metadata, true
}

func newShardProxies(shardHostMap map[string]config.IntervalURLMap, beyondShardProxies Proxies, serviceLogger *logging.ServiceLogger) ShardProxies {
	// create reverse proxy for each backend url
	proxyByURL := make(map[*url.URL]*httputil.ReverseProxy)
	for _, shards := range shardHostMap {
		for _, route := range shards.UrlByEndHeight {
			proxyByURL[route] = httputil.NewSingleHostReverseProxy(route)
		}
	}

	return ShardProxies{
		ServiceLogger:  serviceLogger,
		shardsByHost:   shardHostMap,
		defaultProxies: beyondShardProxies,
		proxyByURL:     proxyByURL,
	}
}
