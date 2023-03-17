package main_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/stretchr/testify/assert"
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

	proxyServiceURL     = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_URL")
	proxyServiceDataURL = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_DATA_URL")

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

	if err != nil {
		t.Fatal(err)
	}

	header, err := client.HeaderByNumber(testContext, nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, int(header.Number.Int64()), 0)
}

func TestE2ETestProxyProxiesForMultipleHosts(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)

	if err != nil {
		t.Fatal(err)
	}

	header, err := client.HeaderByNumber(testContext, nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, int(header.Number.Int64()), 0)

	dataClient, err := ethclient.Dial(proxyServiceDataURL)

	if err != nil {
		t.Fatal(err)
	}

	header, err = dataClient.HeaderByNumber(testContext, nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, int(header.Number.Int64()), 0)
}

func TestE2ETestProxyCreatesRequestMetricForEachRequest(t *testing.T) {
	testEthMethodName := "eth_getBlockByNumber"
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	if err != nil {
		t.Fatal(err)
	}

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	if err != nil {
		t.Fatal(err)
	}

	// make request to api and track start / end time of the request to
	startTime := time.Now()

	_, err = client.HeaderByNumber(testContext, nil)

	endTime := time.Now()

	if err != nil {
		t.Fatal(err)
	}

	// lookup all the request metrics in the database
	// paging as necessary
	var nextCursor int64
	var proxiedRequestMetrics []database.ProxiedRequestMetric

	proxiedRequestMetricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

	if err != nil {
		t.Fatal(err)
	}

	proxiedRequestMetrics = proxiedRequestMetricsPage

	for nextCursor != 0 {
		proxiedRequestMetricsPage, nextCursor, err = database.ListProxiedRequestMetricsWithPagination(testContext, databaseClient.DB, nextCursor, 10000)

		if err != nil {
			t.Fatal(err)
		}

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
}
