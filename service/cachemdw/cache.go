package cachemdw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// ServiceCache is responsible for caching EVM requests and provides corresponding middleware
// ServiceCache can work with any underlying storage which implements simple cache.Cache interface
type ServiceCache struct {
	cacheClient cache.Cache
	blockGetter decode.EVMBlockGetter
	// TTL for cached evm requests
	cacheTTL time.Duration
	// if cacheIndefinitely set to true it overrides cacheTTL and sets TTL to infinity
	cacheIndefinitely        bool
	decodedRequestContextKey any
	// cachePrefix is used as prefix for any key in the cache
	cachePrefix  string
	cacheEnabled bool

	*logging.ServiceLogger
}

func NewServiceCache(
	cacheClient cache.Cache,
	blockGetter decode.EVMBlockGetter,
	cacheTTL time.Duration,
	cacheIndefinitely bool,
	decodedRequestContextKey any,
	cachePrefix string,
	cacheEnabled bool,
	logger *logging.ServiceLogger,
) *ServiceCache {
	return &ServiceCache{
		cacheClient:              cacheClient,
		blockGetter:              blockGetter,
		cacheTTL:                 cacheTTL,
		cacheIndefinitely:        cacheIndefinitely,
		decodedRequestContextKey: decodedRequestContextKey,
		cachePrefix:              cachePrefix,
		cacheEnabled:             cacheEnabled,
		ServiceLogger:            logger,
	}
}

// IsCacheable checks if EVM request is cacheable.
// In current implementation we consider request is cacheable if it has specific block height
func IsCacheable(
	logger *logging.ServiceLogger,
	req *decode.EVMRPCRequestEnvelope,
) bool {
	if req.Method == "" {
		return false
	}

	if decode.MethodHasBlockHashParam(req.Method) {
		return true
	}

	if decode.MethodHasBlockNumberParam(req.Method) {
		blockNumber, err := decode.ParseBlockNumberFromParams(req.Method, req.Params)
		if err != nil {
			logger.Logger.Error().
				Err(err).
				Msg("can't parse block number from params")
			return false
		}

		// blockNumber < 0 means magic tag was used, one of the "latest", "pending", "earliest", etc...
		// we cache requests without magic tag or with the earliest magic tag
		return blockNumber > 0 || blockNumber == decode.BlockTagToNumberCodec[decode.BlockTagEarliest]
	}

	return false
}

// GetCachedQueryResponse calculates cache key for request and then tries to get it from cache.
// NOTE: only JSON-RPC response's result will be taken from the cache.
// JSON-RPC response's ID and Version will be constructed on the fly to match JSON-RPC request.
func (c *ServiceCache) GetCachedQueryResponse(
	ctx context.Context,
	req *decode.EVMRPCRequestEnvelope,
) ([]byte, error) {
	// if request isn't cacheable - there is no point to try to get it from cache so exit early with an error
	cacheable := IsCacheable(c.ServiceLogger, req)
	if !cacheable {
		return nil, ErrRequestIsNotCacheable
	}

	key, err := GetQueryKey(c.cachePrefix, req)
	if err != nil {
		return nil, err
	}

	// get JSON-RPC response's result from the cache
	result, err := c.cacheClient.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	// JSON-RPC response's ID and Version should match JSON-RPC request
	id := strconv.Itoa(int(req.ID))
	response := JsonRpcResponse{
		Version: req.JSONRPCVersion,
		ID:      []byte(id),
		Result:  result,
	}
	responseInJSON, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return responseInJSON, nil
}

// CacheQueryResponse calculates cache key for request and then saves response to the cache.
// NOTE: only JSON-RPC response's result is cached.
// There is no point to cache JSON-RPC response's ID (because it should correspond to request's ID, which constantly changes).
// Same with JSON-RPC response's Version.
func (c *ServiceCache) CacheQueryResponse(
	ctx context.Context,
	req *decode.EVMRPCRequestEnvelope,
	responseInBytes []byte,
) error {
	// don't cache uncacheable requests
	if !IsCacheable(c.ServiceLogger, req) {
		return errors.New("query isn't cacheable")
	}

	response, err := UnmarshalJsonRpcResponse(responseInBytes)
	if err != nil {
		return fmt.Errorf("can't unmarshal json-rpc response: %w", err)
	}
	// don't cache uncacheable responses
	if !response.IsCacheable() {
		return fmt.Errorf("response isn't cacheable")
	}

	key, err := GetQueryKey(c.cachePrefix, req)
	if err != nil {
		return err
	}

	return c.cacheClient.Set(ctx, key, response.Result, c.cacheTTL, c.cacheIndefinitely)
}

func (c *ServiceCache) Healthcheck(ctx context.Context) error {
	return c.cacheClient.Healthcheck(ctx)
}

func (c *ServiceCache) IsCacheEnabled() bool {
	return c.cacheEnabled
}
