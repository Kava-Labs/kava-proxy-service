package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

const (
	EthClientUserAgent = "Go-http-client/1.1"
)

var (
	testContext = context.Background()

	testServiceLogger = func() logging.ServiceLogger {
		logger, err := logging.New("ERROR")
		if err != nil {
			panic(err)

		}
		return logger
	}()

	proxyServiceURL      = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_URL")
	proxyServiceHostname = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_HOSTNAME")
	proxyServiceDataURL  = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_DATA_URL")

	databaseURL      = os.Getenv("TEST_DATABASE_ENDPOINT_URL")
	databasePassword = os.Getenv("DATABASE_PASSWORD")
	databaseUsername = os.Getenv("DATABASE_USERNAME")
	databaseName     = os.Getenv("DATABASE_NAME")
	databaseConfig   = database.PostgresDatabaseConfig{
		DatabaseName:          databaseName,
		DatabaseEndpointURL:   databaseURL,
		DatabaseUsername:      databaseUsername,
		DatabasePassword:      databasePassword,
		SSLEnabled:            false,
		QueryLoggingEnabled:   false,
		Logger:                &testServiceLogger,
		RunDatabaseMigrations: false,
	}

	redisHostPort = os.Getenv("REDIS_HOST_PORT")
	redisPassword = os.Getenv("REDIS_PASSWORD")
)

func TestE2ETestProxyReturnsNonZeroLatestBlockHeader(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	header, err := client.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	assert.Greater(t, int(header.Number.Int64()), 0)
}

func TestE2ETestProxyProxiesForMultipleHosts(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	header, err := client.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	assert.Greater(t, int(header.Number.Int64()), 0)

	dataClient, err := ethclient.Dial(proxyServiceDataURL)

	require.NoError(t, err)

	header, err = dataClient.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	assert.Greater(t, int(header.Number.Int64()), 0)
}

func TestE2ETestProxyCreatesRequestMetricForEachRequest(t *testing.T) {
	testEthMethodName := "eth_getBlockByNumber"
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	require.NoError(t, err)

	// make request to api and track start / end time of the request to
	startTime := time.Now()

	_, err = client.HeaderByNumber(testContext, nil)

	endTime := time.Now()

	require.NoError(t, err)

	// lookup all the request metrics in the database
	// paging as necessary
	var nextCursor int64
	var proxiedRequestMetrics []database.ProxiedRequestMetric

	proxiedRequestMetricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

	require.NoError(t, err)

	proxiedRequestMetrics = proxiedRequestMetricsPage

	for nextCursor != 0 {
		proxiedRequestMetricsPage, nextCursor, err = database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

		require.NoError(t, err)

		proxiedRequestMetrics = append(proxiedRequestMetrics, proxiedRequestMetricsPage...)

	}

	// search for any request metrics during the test timespan
	// with the same method used by the test
	var requestMetricsDuringRequestWindow []database.ProxiedRequestMetric
	// iterate in reverse order to start checking the most request metrics first
	for i := len(proxiedRequestMetrics) - 1; i >= 0; i-- {
		requestMetric := proxiedRequestMetrics[i]
		if requestMetric.RequestTime.After(startTime) && requestMetric.RequestTime.Before(endTime) {
			if requestMetric.MethodName == testEthMethodName {
				requestMetricsDuringRequestWindow = append(requestMetricsDuringRequestWindow, requestMetric)
				break
			}
		}
	}

	assert.Greater(t, len(requestMetricsDuringRequestWindow), 0)

	requestMetricDuringRequestWindow := requestMetricsDuringRequestWindow[0]

	assert.Greater(t, requestMetricDuringRequestWindow.ResponseLatencyMilliseconds, int64(0))
	assert.Equal(t, requestMetricDuringRequestWindow.MethodName, testEthMethodName)
	assert.Equal(t, requestMetricDuringRequestWindow.Hostname, proxyServiceHostname)
	assert.NotEqual(t, requestMetricDuringRequestWindow.RequestIP, "")
	assert.Equal(t, *requestMetricDuringRequestWindow.UserAgent, EthClientUserAgent)
	assert.NotEqual(t, *requestMetricDuringRequestWindow.Referer, "")
	assert.NotEqual(t, *requestMetricDuringRequestWindow.Origin, "")
}

func TestE2ETestProxyTracksBlockNumberForEth_getBlockByNumberRequest(t *testing.T) {
	testEthMethodName := "eth_getBlockByNumber"

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	require.NoError(t, err)

	// get the latest queryable block number
	// need to do this dynamically since not all blocks
	// are queryable for a given network
	response, err := client.HeaderByNumber(testContext, nil)

	require.NoError(t, err)

	requestBlockNumber := response.Number

	// make request to api and track start / end time of the request to
	startTime := time.Now()

	_, err = client.HeaderByNumber(testContext, requestBlockNumber)

	endTime := time.Now()

	require.NoError(t, err)

	// lookup all the request metrics in the database
	// paging as necessary
	var nextCursor int64
	var proxiedRequestMetrics []database.ProxiedRequestMetric

	proxiedRequestMetricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

	require.NoError(t, err)

	proxiedRequestMetrics = proxiedRequestMetricsPage

	for nextCursor != 0 {
		proxiedRequestMetricsPage, nextCursor, err = database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

		require.NoError(t, err)

		proxiedRequestMetrics = append(proxiedRequestMetrics, proxiedRequestMetricsPage...)

	}

	// search for any request metrics during the test timespan
	// with the same method used by the test
	var requestMetricsDuringRequestWindow []database.ProxiedRequestMetric
	// iterate in reverse order to start checking the most recent request metrics first
	for i := len(proxiedRequestMetrics) - 1; i >= 0; i-- {
		requestMetric := proxiedRequestMetrics[i]
		if requestMetric.RequestTime.After(startTime) && requestMetric.RequestTime.Before(endTime) {
			if requestMetric.MethodName == testEthMethodName {
				requestMetricsDuringRequestWindow = append(requestMetricsDuringRequestWindow, requestMetric)
				break
			}
		}
	}

	assert.Greater(t, len(requestMetricsDuringRequestWindow), 0)
	requestMetricDuringRequestWindow := requestMetricsDuringRequestWindow[0]

	assert.Equal(t, requestMetricDuringRequestWindow.MethodName, testEthMethodName)
	assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
	assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, requestBlockNumber.Int64())
}

func TestE2ETestProxyTracksBlockTagForEth_getBlockByNumberRequest(t *testing.T) {
	testEthMethodName := "eth_getBlockByNumber"
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	require.NoError(t, err)

	// make request to api and track start / end time of the request to
	startTime := time.Now()

	// will default to latest
	_, err = client.HeaderByNumber(testContext, nil)

	endTime := time.Now()

	require.NoError(t, err)

	// lookup all the request metrics in the database
	// paging as necessary
	var nextCursor int64
	var proxiedRequestMetrics []database.ProxiedRequestMetric

	proxiedRequestMetricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

	require.NoError(t, err)

	proxiedRequestMetrics = proxiedRequestMetricsPage

	for nextCursor != 0 {
		proxiedRequestMetricsPage, nextCursor, err = database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

		require.NoError(t, err)

		proxiedRequestMetrics = append(proxiedRequestMetrics, proxiedRequestMetricsPage...)

	}

	// search for any request metrics during the test timespan
	// with the same method used by the test
	var requestMetricsDuringRequestWindow []database.ProxiedRequestMetric
	// iterate in reverse order to start checking the most request metrics first
	for i := len(proxiedRequestMetrics) - 1; i >= 0; i-- {
		requestMetric := proxiedRequestMetrics[i]
		if requestMetric.RequestTime.After(startTime) && requestMetric.RequestTime.Before(endTime) {
			if requestMetric.MethodName == testEthMethodName {
				requestMetricsDuringRequestWindow = append(requestMetricsDuringRequestWindow, requestMetric)
				break
			}
		}
	}

	assert.Greater(t, len(requestMetricsDuringRequestWindow), 0)
	requestMetricDuringRequestWindow := requestMetricsDuringRequestWindow[0]

	assert.Equal(t, requestMetricDuringRequestWindow.MethodName, testEthMethodName)
	assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
	assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, decode.BlockTagToNumberCodec["latest"])
}

func TestE2ETestProxyTracksBlockNumberForMethodsWithBlockNumberParam(t *testing.T) {
	testedmethods := decode.CacheableByBlockNumberMethods
	testRandomAddressHex := "0x6767114FFAA17C6439D7AEA480738B982CE63A02"
	testAddress := common.HexToAddress(testRandomAddressHex)
	testRandomHash := common.HexToHash(testRandomAddressHex)

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)
	require.NoError(t, err)

	// get the latest queryable block number
	// need to do this dynamically since not all blocks
	// are queryable for a given network
	latestBlock, err := client.HeaderByNumber(testContext, nil)

	require.NoError(t, err)

	requestBlockNumber := latestBlock.Number

	// make requests to api and track start / end time of the request
	// we don't check response errors because the proxy will create metrics
	// for each request whether the kava node api returns an error or not
	// and if it doesn't the test itself will fail due to missing metrics
	startTime := time.Now()

	// eth_getBalance
	_, _ = client.BalanceAt(testContext, testAddress, requestBlockNumber)

	// eth_getStorageAt
	_, _ = client.StorageAt(testContext, testAddress, testRandomHash, requestBlockNumber)

	// eth_getTransactionCount
	_, _ = client.NonceAt(testContext, testAddress, requestBlockNumber)

	// eth_getBlockTransactionCountByNumber
	_, _ = client.PendingTransactionCount(testContext)

	// eth_getCode
	_, _ = client.CodeAt(testContext, testAddress, requestBlockNumber)

	// eth_getBlockByNumber
	_, _ = client.HeaderByNumber(testContext, requestBlockNumber)

	// eth_call
	_, _ = client.CallContract(testContext, ethereum.CallMsg{}, requestBlockNumber)

	// plus a buffer for slower connections (amd64 lol) :)
	endTime := time.Now().Add(10)

	// lookup all the request metrics in the database
	// paging as necessary
	var nextCursor int64
	var proxiedRequestMetrics []database.ProxiedRequestMetric

	proxiedRequestMetricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

	require.NoError(t, err)

	proxiedRequestMetrics = proxiedRequestMetricsPage

	for nextCursor != 0 {
		proxiedRequestMetricsPage, nextCursor, err = database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

		require.NoError(t, err)

		proxiedRequestMetrics = append(proxiedRequestMetrics, proxiedRequestMetricsPage...)

	}

	// search for any request metrics during the test timespan
	// with the same method used by the test
	var requestMetricsDuringRequestWindow []database.ProxiedRequestMetric
	// iterate in reverse order to start checking the most recent request metrics first
	for i := len(proxiedRequestMetrics) - 1; i >= 0; i-- {
		requestMetric := proxiedRequestMetrics[i]
		if requestMetric.RequestTime.After(startTime) && requestMetric.RequestTime.Before(endTime) {
			for _, testedMethod := range testedmethods {
				if requestMetric.MethodName == testedMethod {
					requestMetricsDuringRequestWindow = append(requestMetricsDuringRequestWindow, requestMetric)
				}
			}
		}
	}

	// assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), len(testedmethods))
	// should be the above but geth doesn't implement client methods for all of them
	assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), 7)

	for _, requestMetricDuringRequestWindow := range requestMetricsDuringRequestWindow {
		assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
		if requestMetricDuringRequestWindow.MethodName == "eth_getBlockTransactionCountByNumber" {
			assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, decode.BlockTagToNumberCodec["pending"])
			continue
		}
		assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, requestBlockNumber.Int64())
	}
}

func TestE2ETestProxyTracksBlockNumberForMethodsWithBlockHashParam(t *testing.T) {
	testedmethods := decode.CacheableByBlockHashMethods

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	require.NoError(t, err)

	// get the latest queryable block number
	// need to do this dynamically since not all blocks
	// are queryable for a given network
	latestBlock, err := client.HeaderByNumber(testContext, nil)

	require.NoError(t, err)

	requestBlockHash := latestBlock.ParentHash
	// minus one since we are looking up the parent block
	requestBlockNumber := latestBlock.Number.Int64() - 1

	// make requests to api and track start / end time of the request
	// we don't check response errors because the proxy will create metrics
	// for each request whether the kava node api returns an error or not
	// and if it doesn't the test itself will fail due to missing metrics
	startTime := time.Now()

	// eth_getBlockByHash
	_, _ = client.BlockByHash(testContext, requestBlockHash)

	// eth_getBlockTransactionCountByHash
	_, _ = client.TransactionCount(testContext, requestBlockHash)

	// eth_getTransactionByBlockHashAndIndex
	_, _ = client.TransactionInBlock(testContext, requestBlockHash, 0)
	endTime := time.Now()

	// lookup all the request metrics in the database
	// paging as necessary
	var nextCursor int64
	var proxiedRequestMetrics []database.ProxiedRequestMetric

	proxiedRequestMetricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

	require.NoError(t, err)

	proxiedRequestMetrics = proxiedRequestMetricsPage

	for nextCursor != 0 {
		proxiedRequestMetricsPage, nextCursor, err = database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

		require.NoError(t, err)

		proxiedRequestMetrics = append(proxiedRequestMetrics, proxiedRequestMetricsPage...)

	}

	// search for any request metrics during the test timespan
	// with the same method used by the test
	var requestMetricsDuringRequestWindow []database.ProxiedRequestMetric
	// iterate in reverse order to start checking the most recent request metrics first
	for i := len(proxiedRequestMetrics) - 1; i >= 0; i-- {
		requestMetric := proxiedRequestMetrics[i]
		if requestMetric.RequestTime.After(startTime) && requestMetric.RequestTime.Before(endTime) {
			for _, testedMethod := range testedmethods {
				if requestMetric.MethodName == testedMethod {
					requestMetricsDuringRequestWindow = append(requestMetricsDuringRequestWindow, requestMetric)
				}
			}
		}
	}

	// assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), len(testedmethods))
	// should be the above but geth doesn't implement client methods for all of them
	assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), 3)

	for _, requestMetricDuringRequestWindow := range requestMetricsDuringRequestWindow {
		assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
		assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, requestBlockNumber)
	}
}

func TestE2eTestCachingMdwWithBlockNumberParam(t *testing.T) {
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	if err != nil {
		t.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%v", redisHostPort),
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc    string
		method  string
		params  []interface{}
		keysNum int
	}{
		{
			desc:    "test case #1",
			method:  "eth_getBlockByNumber",
			params:  []interface{}{"0x1", true},
			keysNum: 1,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// test cache MISS and cache HIT scenarios for specified method
			// check corresponding values in cachemdw.CacheMissHeaderValue HTTP header
			// check that cached and non-cached responses are equal

			// eth_getBlockByNumber - cache MISS
			resp1 := mkJsonRpcRequest(t, proxyServiceURL, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp1.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)

			// eth_getBlockByNumber - cache HIT
			resp2 := mkJsonRpcRequest(t, proxyServiceURL, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheHitHeaderValue, resp2.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)

			require.JSONEq(t, string(body1), string(body2), "blocks should be the same")
		})
	}

	// test cache MISS and cache HIT scenarios for eth_getBlockByNumber method
	// check that cached and non-cached responses are equal
	{
		// eth_getBlockByNumber - cache MISS
		block1, err := client.BlockByNumber(testContext, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 2)

		// eth_getBlockByNumber - cache HIT
		block2, err := client.BlockByNumber(testContext, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 2)

		require.Equal(t, block1, block2, "blocks should be the same")
	}

	cleanUpRedis(t, redisClient)
}

func TestE2eTestCachingMdwWithBlockNumberParam_EmptyResult(t *testing.T) {
	testRandomAddressHex := "0x6767114FFAA17C6439D7AEA480738B982CE63A02"
	testAddress := common.HexToAddress(testRandomAddressHex)

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	if err != nil {
		t.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%v", redisHostPort),
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc    string
		method  string
		params  []interface{}
		keysNum int
	}{
		{
			desc:    "test case #1",
			method:  "eth_getTransactionCount",
			params:  []interface{}{testAddress, "0x1"},
			keysNum: 0,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// both calls should lead to cache MISS scenario, because empty results aren't cached
			// check corresponding values in cachemdw.CacheMissHeaderValue HTTP header
			// check that responses are equal

			// eth_getBlockByNumber - cache MISS
			resp1 := mkJsonRpcRequest(t, proxyServiceURL, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp1.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)

			// eth_getBlockByNumber - cache MISS again (empty results aren't cached)
			resp2 := mkJsonRpcRequest(t, proxyServiceURL, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp2.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)

			require.JSONEq(t, string(body1), string(body2), "blocks should be the same")
		})
	}

	// both calls should lead to cache MISS scenario, because empty results aren't cached
	// check that responses are equal
	{
		// eth_getTransactionCount - cache MISS
		bal1, err := client.NonceAt(testContext, testAddress, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 0)

		// eth_getTransactionCount - cache MISS again (empty results aren't cached)
		bal2, err := client.NonceAt(testContext, testAddress, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 0)

		require.Equal(t, bal1, bal2, "balances should be the same")
	}

	cleanUpRedis(t, redisClient)
}

func expectKeysNum(t *testing.T, redisClient *redis.Client, keysNum int) {
	keys, err := redisClient.Keys(context.Background(), "*").Result()
	require.NoError(t, err)

	require.Equal(t, keysNum, len(keys))
}

func cleanUpRedis(t *testing.T, redisClient *redis.Client) {
	keys, err := redisClient.Keys(context.Background(), "*").Result()
	require.NoError(t, err)

	if len(keys) != 0 {
		_, err = redisClient.Del(context.Background(), keys...).Result()
		require.NoError(t, err)
	}
}

func mkJsonRpcRequest(t *testing.T, proxyServiceURL, method string, params []interface{}) *http.Response {
	req := newJsonRpcRequest(method, params)
	reqInJSON, err := json.Marshal(req)
	require.NoError(t, err)
	reqReader := bytes.NewBuffer(reqInJSON)
	resp, err := http.Post(proxyServiceURL, "application/json", reqReader)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	return resp
}

type jsonRpcRequest struct {
	JsonRpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      int           `json:"id"`
}

func newJsonRpcRequest(method string, params []interface{}) *jsonRpcRequest {
	return &jsonRpcRequest{
		JsonRpc: "2.0",
		Method:  method,
		Params:  params,
		Id:      1,
	}
}

type jsonRpcResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	Id      int         `json:"id"`
	Result  interface{} `json:"result"`
	Error   string      `json:"error"`
}

func checkJsonRpcErr(body []byte) error {
	var resp jsonRpcResponse
	err := json.Unmarshal(body, &resp)
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return errors.New(resp.Error)
	}

	if resp.Result == "" {
		return errors.New("result is empty")
	}

	return nil
}
