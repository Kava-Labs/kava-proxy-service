package cachemdw

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// ServiceCache is responsible for caching EVM requests and provides corresponding middleware
// ServiceCache can work with any underlying storage which implements simple cache.Cache interface
type ServiceCache struct {
	cacheClient              cache.Cache
	blockGetter              decode.EVMBlockGetter
	cacheTTL                 time.Duration
	decodedRequestContextKey any
	// chainID is used as prefix for any key in the cache
	chainID string

	*logging.ServiceLogger
}

func NewServiceCache(
	cacheClient cache.Cache,
	blockGetter decode.EVMBlockGetter,
	cacheTTL time.Duration,
	decodedRequestContextKey any,
	chainID string,
	logger *logging.ServiceLogger,
) *ServiceCache {
	return &ServiceCache{
		cacheClient:              cacheClient,
		blockGetter:              blockGetter,
		cacheTTL:                 cacheTTL,
		decodedRequestContextKey: decodedRequestContextKey,
		chainID:                  chainID,
		ServiceLogger:            logger,
	}
}

// IsCacheable checks if EVM request is cacheable.
// In current implementation we consider request is cacheable if it has specific block height
func IsCacheable(
	ctx context.Context,
	blockGetter decode.EVMBlockGetter,
	logger *logging.ServiceLogger,
	req *decode.EVMRPCRequestEnvelope,
) bool {
	blockNumber, err := req.ExtractBlockNumberFromEVMRPCRequest(ctx, blockGetter)
	if err != nil {
		logger.Logger.Error().
			Err(err).
			Msg("can't extract block number from EVM RPC request")
		return false
	}

	// blockNumber <= 0 means magic tag was used, one of the "latest", "pending", "earliest", etc...
	// as of now we don't cache requests with magic tags
	if blockNumber <= 0 {
		return false
	}

	// block number is specified and it's not a magic tag - cache the request
	return true
}

// GetCachedQueryResponse calculates cache key for request and then tries to get it from cache.
func (c *ServiceCache) GetCachedQueryResponse(
	ctx context.Context,
	req *decode.EVMRPCRequestEnvelope,
) ([]byte, error) {
	key, err := GetQueryKey(c.chainID, req)
	if err != nil {
		return nil, err
	}

	value, err := c.cacheClient.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// CacheQueryResponse calculates cache key for request and then saves response to the cache.
func (c *ServiceCache) CacheQueryResponse(
	ctx context.Context,
	req *decode.EVMRPCRequestEnvelope,
	chainID string,
	response []byte,
) error {
	if !IsCacheable(ctx, c.blockGetter, c.ServiceLogger, req) {
		return errors.New("query isn't cacheable")
	}

	key, err := GetQueryKey(chainID, req)
	if err != nil {
		return err
	}

	return c.cacheClient.Set(ctx, key, response, c.cacheTTL)
}

func (c *ServiceCache) ValidateAndCacheQueryResponse(
	ctx context.Context,
	req *decode.EVMRPCRequestEnvelope,
	responseInBytes []byte,
) error {
	response, err := UnmarshalJsonRpcResponse(responseInBytes)
	if err != nil {
		return fmt.Errorf("can't unmarshal json-rpc response: %w", err)
	}
	// don't cache uncacheable responses
	if !response.IsCacheable() {
		return fmt.Errorf("response isn't cacheable")
	}

	if err := c.CacheQueryResponse(
		ctx,
		req,
		c.chainID,
		responseInBytes,
	); err != nil {
		return err
	}

	return nil
}

func (c *ServiceCache) Healthcheck(ctx context.Context) error {
	return c.cacheClient.Healthcheck(ctx)
}
