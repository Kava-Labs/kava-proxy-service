package main_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
