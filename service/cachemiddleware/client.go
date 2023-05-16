package cachemiddleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// CacheClient is a cache client for requesting cacheable data.
type CacheClient struct {
	cache                    cache.Cache       // cache is the cache implementation used to fetch cached data
	evmClient                *ethclient.Client // evmClient is the eth client used to fetch required evm data for caching
	cacheTTL                 time.Duration     // cacheTTL is the expiration duration for cached data
	decodedRequestContextKey any               // decodedRequestContextKey is the context key for the decoded request

	logger *logging.ServiceLogger
}

// NewClient returns a new CacheClient client.
func NewClient(
	cache cache.Cache,
	evmClient *ethclient.Client,
	cacheTTL time.Duration,
	decodedRequestContextKey any,
	logger *logging.ServiceLogger,
) *CacheClient {
	return &CacheClient{
		cache:                    cache,
		evmClient:                evmClient,
		cacheTTL:                 cacheTTL,
		decodedRequestContextKey: decodedRequestContextKey,
		logger:                   logger,
	}
}

// getChainIDFromHost returns the chain ID for the given http request host. This
// will attempt to fetch the chain ID from the cache. If the chain ID is not
// found, an error will be returned.
func (c *CacheClient) getChainIDFromHost(
	ctx context.Context,
	host string,
) (chainID string, found bool) {
	key := GetChainKey(host)

	bytes, found := c.cache.Get(ctx, key)
	if found {
		return string(bytes), true
	}

	return "", false
}

func (c *CacheClient) setChainIDForHost(
	ctx context.Context,
	host string,
	chainID string,
) error {
	key := GetChainKey(host)

	c.logger.Debug().Str("key", key).Msg("caching host chain ID")

	return c.cache.Set(ctx, key, []byte(chainID), c.cacheTTL)
}

// GetCachedRequest returns the cached request for the given http request and
// decoded request envelope. This will attempt to fetch the request from the
// cache, and will return an error if the request is not found.
func (c *CacheClient) GetCachedRequest(
	ctx context.Context,
	r *http.Request,
	decodedReq *decode.EVMRPCRequestEnvelope,
) (data []byte, found bool, shouldCache bool) {
	// TODO: We may want to have a different set of cacheable methods
	// that is a smaller list than ExtractBlockNumberFromEVMRPCRequest uses.
	// Skip caching if we can't extract block number
	blockNumber, err := decodedReq.ExtractBlockNumberFromEVMRPCRequest(ctx, c.evmClient)
	if err != nil {
		return nil, false, false
	}

	if blockNumber <= 0 {
		// Don't cache requests that don't have a block number
		return nil, false, false
	}

	chainID, found := c.getChainIDFromHost(ctx, r.Host)
	if !found {
		return nil, false, true
	}

	key, err := GetQueryKey(r, chainID, decodedReq)
	if err != nil {
		// Don't cache requests that fail to build a cache key
		return nil, false, false
	}

	bytes, found := c.cache.Get(ctx, key)
	if !found {
		// Cache requests that are not found
		return nil, false, true
	}

	c.logger.Debug().Msg(fmt.Sprintf("found cached response for key %s", key))

	// Not necessary to re-cache requests that are found in cache
	return bytes, false, false
}

// SetCachedRequest sets the cached request for the given http request and
// decoded request envelope.
func (c *CacheClient) SetCachedRequest(
	ctx context.Context,
	r *http.Request,
	decodedReq *decode.EVMRPCRequestEnvelope,
	resp []byte,
) error {
	blockNumber, err := decodedReq.ExtractBlockNumberFromEVMRPCRequest(ctx, c.evmClient)
	if err != nil {
		return fmt.Errorf(
			"error extracting block number from request for method: %s",
			decodedReq.Method,
		)
	}

	if blockNumber <= 0 {
		return fmt.Errorf("block number is not positive %d", blockNumber)
	}

	chainID, found := c.getChainIDFromHost(ctx, r.Host)
	if !found {
		// Fetch the chain ID for the host if it is not found in cache
		rawChainID, err := c.evmClient.ChainID(ctx)
		if err != nil {
			return fmt.Errorf("error getting chain ID: %w", err)
		}

		// Cache the chain ID for the host
		err = c.setChainIDForHost(ctx, r.Host, rawChainID.String())
		if err != nil {
			return fmt.Errorf("error setting chain ID for host: %w", err)
		}

		// Update the chain ID
		chainID = rawChainID.String()
	}

	key, err := GetQueryKey(r, chainID, decodedReq)
	if err != nil {
		return err
	}

	c.logger.Debug().Str("key", key).Msg("caching response")
	return c.cache.Set(ctx, key, resp, c.cacheTTL)
}
