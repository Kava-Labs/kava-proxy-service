package main_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service"
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

	proxyServiceURL        = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_URL")
	proxyServiceHostname   = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_HOSTNAME")
	proxyServicePruningURL = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_PRUNING_URL")

	proxyServiceHeightBasedRouting, _ = strconv.ParseBool(os.Getenv("TEST_PROXY_HEIGHT_BASED_ROUTING_ENABLED"))

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

// lookup all the request metrics in the database paging as necessary
// search for any request metrics during the time window for particular request methods
func findMetricsInWindowForMethods(db database.PostgresClient, startTime time.Time, endTime time.Time, testedmethods []string) []database.ProxiedRequestMetric {
	var nextCursor int64
	var proxiedRequestMetrics []database.ProxiedRequestMetric

	proxiedRequestMetricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(testContext, db.DB, nextCursor, 10000)
	if err != nil {
		panic(err)
	}

	proxiedRequestMetrics = proxiedRequestMetricsPage
	for nextCursor != 0 {
		proxiedRequestMetricsPage, nextCursor, err = database.ListProxiedRequestMetricsWithPagination(testContext, db.DB, nextCursor, 10000)
		if err != nil {
			panic(err)
		}

		proxiedRequestMetrics = append(proxiedRequestMetrics, proxiedRequestMetricsPage...)
	}

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

	return requestMetricsDuringRequestWindow
}

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

	pruningClient, err := ethclient.Dial(proxyServicePruningURL)

	require.NoError(t, err)

	header, err = pruningClient.HeaderByNumber(testContext, nil)
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

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, []string{testEthMethodName})

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

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, []string{testEthMethodName})

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

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, []string{testEthMethodName})

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

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, testedmethods)

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

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, testedmethods)

	// assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), len(testedmethods))
	// should be the above but geth doesn't implement client methods for all of them
	assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), 3)

	for _, requestMetricDuringRequestWindow := range requestMetricsDuringRequestWindow {
		assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
		assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, requestBlockNumber)
	}
}

func TestE2ETest_HeightBasedRouting(t *testing.T) {
	if !proxyServiceHeightBasedRouting {
		t.Skip("TEST_PROXY_HEIGHT_BASED_ROUTING_ENABLED is false. skipping height-based routing e2e test")
	}

	rpc, err := rpc.Dial(proxyServiceURL)
	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)
	require.NoError(t, err)

	testCases := []struct {
		name        string
		method      string
		params      []interface{}
		expectRoute string
	}{
		{
			name:        "request for non-latest height -> default",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"0x2", false},
			expectRoute: service.ResponseBackendDefault,
		},
		{
			name:        "request for earliest height -> default",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"earliest", false},
			expectRoute: service.ResponseBackendDefault,
		},
		{
			name:        "request for latest height -> pruning",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"latest", false},
			expectRoute: service.ResponseBackendPruning,
		},
		{
			name:        "request for finalized height -> pruning",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"finalized", false},
			expectRoute: service.ResponseBackendPruning,
		},
		{
			name:        "request with empty height -> pruning",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{nil, false},
			expectRoute: service.ResponseBackendPruning,
		},
		{
			name:        "request not requiring height -> pruning",
			method:      "eth_chainId",
			params:      []interface{}{},
			expectRoute: service.ResponseBackendPruning,
		},
		{
			name:        "request by hash -> default",
			method:      "eth_getBlockByHash",
			params:      []interface{}{"0xe9bd10bc1d62b4406dd1fb3dbf3adb54f640bdb9ebbe3dd6dfc6bcc059275e54", false},
			expectRoute: service.ResponseBackendDefault,
		},
		{
			name:        "un-parseable (invalid) height -> default",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"not-a-block-tag", false},
			expectRoute: service.ResponseBackendDefault,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()
			err := rpc.Call(nil, tc.method, tc.params...)
			require.NoError(t, err)

			metrics := findMetricsInWindowForMethods(databaseClient, startTime, time.Now(), []string{tc.method})

			require.Len(t, metrics, 1)
			fmt.Printf("%+v\n", metrics[0])
			require.Equal(t, metrics[0].MethodName, tc.method)
			require.Equal(t, metrics[0].ResponseBackend, tc.expectRoute)
		})
	}
}
