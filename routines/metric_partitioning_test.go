package routines

import (
	"context"
	"github.com/kava-labs/kava-proxy-service/clients/database/postgres"
	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

var (
	testCtx                                       = context.Background()
	MetricPartitioningRoutineDelayFirstRunSeconds = config.EnvOrDefaultInt(config.METRIC_PARTITIONING_ROUTINE_DELAY_FIRST_RUN_SECONDS_ENVIRONMENT_KEY, config.DEFAULT_METRIC_PARTITIONING_ROUTINE_DELAY_FIRST_RUN_SECONDS)
	proxyServiceURL                               = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_URL")
	configuredPrefillDays                         = config.EnvOrDefaultInt64(config.METRIC_PARTITIONING_PREFILL_PERIOD_DAYS_ENVIRONMENT_KEY, config.DEFAULT_METRIC_PARTITIONING_PREFILL_PERIOD_DAYS)

	proxyServiceClient = func() *service.ProxyServiceClient {
		client, err := service.NewProxyServiceClient(service.ProxyServiceClientConfig{
			ProxyServiceHostname: proxyServiceURL,
		})

		if err != nil {
			panic(err)
		}

		return client
	}()
)

func TestE2ETestMetricPartitioningRoutinePrefillsExpectedPartitionsAfterStartupDelay(t *testing.T) {
	if shouldSkipMetrics() {
		t.Skip("Skipping test because environment variable SKIP_METRICS is set to true")
	}

	// prepare
	time.Sleep(time.Duration(MetricPartitioningRoutineDelayFirstRunSeconds) * time.Second)

	expectedPartitions, err := postgres.PartitionsForPeriod(time.Now().UTC(), int(configuredPrefillDays))

	assert.Nil(t, err)

	// execute
	databaseStatus, err := proxyServiceClient.GetDatabaseStatus(testCtx)

	// assert
	assert.Nil(t, err)
	assert.GreaterOrEqual(t, databaseStatus.TotalProxiedRequestMetricPartitions, configuredPrefillDays)
	assert.Equal(t, expectedPartitions[len(expectedPartitions)-1].TableName, databaseStatus.LatestProxiedRequestMetricPartitionTableName)
}

func shouldSkipMetrics() bool {
	// Check if the environment variable SKIP_METRICS is set to "true"
	return os.Getenv("SKIP_METRICS") == "true"
}
