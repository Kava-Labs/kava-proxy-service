package cachemdw

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

type Config struct {
	// TTL for cached evm requests
	// Different evm method groups may have different TTLs
	// TTL should be either greater than zero or equal to -1, -1 means cache indefinitely
	CacheMethodHasBlockNumberParamTTL time.Duration
	CacheMethodHasBlockHashParamTTL   time.Duration
	CacheStaticMethodTTL              time.Duration
	CacheMethodHasTxHashParamTTL      time.Duration
}

// ServiceCache is responsible for caching EVM requests and provides corresponding middleware
// ServiceCache can work with any underlying storage which implements simple cache.Cache interface
type ServiceCache struct {
	cacheClient              cache.Cache
	blockGetter              decode.EVMBlockGetter
	decodedRequestContextKey any
	// cachePrefix is used as prefix for any key in the cache
	cachePrefix        string
	cacheEnabled       bool
	whitelistedHeaders []string

	defaultAccessControlAllowOriginValue       string
	hostnameToAccessControlAllowOriginValueMap map[string]string

	config *Config

	*logging.ServiceLogger
}

func NewServiceCache(
	cacheClient cache.Cache,
	blockGetter decode.EVMBlockGetter,
	decodedRequestContextKey any,
	cachePrefix string,
	cacheEnabled bool,
	whitelistedHeaders []string,
	defaultAccessControlAllowOriginValue string,
	hostnameToAccessControlAllowOriginValueMap map[string]string,
	config *Config,
	logger *logging.ServiceLogger,
) *ServiceCache {
	return &ServiceCache{
		cacheClient:                          cacheClient,
		blockGetter:                          blockGetter,
		decodedRequestContextKey:             decodedRequestContextKey,
		cachePrefix:                          cachePrefix,
		cacheEnabled:                         cacheEnabled,
		whitelistedHeaders:                   whitelistedHeaders,
		defaultAccessControlAllowOriginValue: defaultAccessControlAllowOriginValue,
		hostnameToAccessControlAllowOriginValueMap: hostnameToAccessControlAllowOriginValueMap,
		config:        config,
		ServiceLogger: logger,
	}
}

// QueryResponse represents the structure which stored in the cache for every cacheable request
type QueryResponse struct {
	// JsonRpcResponseResult is an EVM JSON-RPC response's result
	JsonRpcResponseResult []byte `json:"json_rpc_response_result"`
	// HeaderMap is a map of HTTP headers which is cached along with the EVM JSON-RPC response
	HeaderMap map[string]string `json:"header_map"`
}

// IsCacheable checks if EVM request is cacheable.
// In current implementation we consider request is cacheable if it has specific block height
func IsCacheable(
	logger *logging.ServiceLogger,
	req *decode.EVMRPCRequestEnvelope,
) bool {
	// TODO: technically, we _could_ cache the "invalid request" response for `null` requests...
	if req == nil {
		return false
	}

	if req.Method == "" {
		return false
	}

	if decode.MethodHasBlockNumberParam(req.Method) {
		blockNumber, err := decode.ParseBlockNumberFromParams(req.Method, req.Params)
		if err != nil {
			paramsInJSON, marshalErr := json.Marshal(req.Params)
			if marshalErr != nil {
				logger.Logger.Error().
					Err(marshalErr).
					Msg("can't marshal EVM request params into json")
			}

			logger.Logger.Error().
				Str("method", req.Method).
				Str("params", string(paramsInJSON)).
				Err(err).
				Msg("can't parse block number from params")
			return false
		}

		// blockNumber < 0 means magic tag was used, one of the "latest", "pending", "earliest", etc...
		// we cache requests without magic tag or with the earliest magic tag
		return blockNumber > 0 || blockNumber == decode.BlockTagToNumberCodec[decode.BlockTagEarliest]
	}

	if decode.MethodHasBlockHashParam(req.Method) {
		return true
	}

	if decode.IsMethodStatic(req.Method) {
		return true
	}

	if decode.MethodHasTxHashParam(req.Method) {
		return true
	}

	return false
}

// GetTTL returns TTL for specified EVM method.
func (c *ServiceCache) GetTTL(method string) (time.Duration, error) {
	if decode.MethodHasBlockNumberParam(method) {
		return c.config.CacheMethodHasBlockNumberParamTTL, nil
	}

	if decode.MethodHasBlockHashParam(method) {
		return c.config.CacheMethodHasBlockHashParamTTL, nil
	}

	if decode.IsMethodStatic(method) {
		return c.config.CacheStaticMethodTTL, nil
	}

	if decode.MethodHasTxHashParam(method) {
		return c.config.CacheMethodHasTxHashParamTTL, nil
	}

	return 0, ErrRequestIsNotCacheable
}

// GetCachedQueryResponse calculates cache key for request and then tries to get it from cache.
// NOTE: only JSON-RPC response's result will be taken from the cache.
// JSON-RPC response's ID and Version will be constructed on the fly to match JSON-RPC request.
func (c *ServiceCache) GetCachedQueryResponse(
	ctx context.Context,
	req *decode.EVMRPCRequestEnvelope,
) (*QueryResponse, error) {
	// if request isn't cacheable - there is no point to try to get it from cache so exit early with an error
	cacheable := IsCacheable(c.ServiceLogger, req)
	if !cacheable {
		return nil, ErrRequestIsNotCacheable
	}

	key, err := GetQueryKey(c.cachePrefix, req)
	if err != nil {
		return nil, err
	}

	// get Query Response from the cache
	queryResponseInJSON, err := c.cacheClient.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	// Query Response consists of JSON-RPC response's result and headers map.
	// Unmarshal it and later update JSON-RPC response's result to match JSON-RPC request.
	var queryResponse QueryResponse
	if err := json.Unmarshal(queryResponseInJSON, &queryResponse); err != nil {
		return nil, err
	}

	// JSON-RPC response's ID and Version should match JSON-RPC request
	id, err := json.Marshal(req.ID)
	if err != nil {
		return nil, err
	}
	response := JsonRpcResponse{
		Version: req.JSONRPCVersion,
		ID:      id,
		Result:  queryResponse.JsonRpcResponseResult,
	}
	responseInJSON, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	responseInJSON = append(responseInJSON, '\n')

	// update JSON-RPC response's result before returning Query Response
	queryResponse.JsonRpcResponseResult = responseInJSON

	return &queryResponse, nil
}

// CacheQueryResponse calculates cache key for request and then saves response to the cache.
// NOTE: only JSON-RPC response's result is cached.
// There is no point to cache JSON-RPC response's ID (because it should correspond to request's ID, which constantly changes).
// Same with JSON-RPC response's Version.
func (c *ServiceCache) CacheQueryResponse(
	ctx context.Context,
	req *decode.EVMRPCRequestEnvelope,
	responseInBytes []byte,
	headerMap map[string]string,
) error {
	// don't cache uncacheable requests
	if !IsCacheable(c.ServiceLogger, req) {
		return ErrRequestIsNotCacheable
	}

	response, err := UnmarshalJsonRpcResponse(responseInBytes)
	if err != nil {
		return fmt.Errorf("can't unmarshal json-rpc response: %w", err)
	}
	// don't cache uncacheable responses
	if !response.IsCacheable() {
		return ErrResponseIsNotCacheable
	}
	if !response.IsFinal(req.Method) {
		return ErrResponseIsNotFinal
	}

	key, err := GetQueryKey(c.cachePrefix, req)
	if err != nil {
		return err
	}

	// cache JSON-RPC response's result and HTTP Header Map
	queryResponse := &QueryResponse{
		JsonRpcResponseResult: response.Result,
		HeaderMap:             headerMap,
	}
	queryResponseInJSON, err := json.Marshal(queryResponse)
	if err != nil {
		return err
	}

	cacheTTL, err := c.GetTTL(req.Method)
	if err != nil {
		return fmt.Errorf("can't get cache TTL for %v method: %v", req.Method, err)
	}

	return c.cacheClient.Set(ctx, key, queryResponseInJSON, cacheTTL)
}

func (c *ServiceCache) Healthcheck(ctx context.Context) error {
	return c.cacheClient.Healthcheck(ctx)
}

func (c *ServiceCache) IsCacheEnabled() bool {
	return c.cacheEnabled
}
