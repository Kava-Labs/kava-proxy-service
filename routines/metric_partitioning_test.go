package routines

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/stretchr/testify/assert"
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

// func TestE2ETestMetricPartitioningRoutinePrefillsExpectedPartitionsAfterStartupDelay(t *testing.T) {
// 	// prepare
// 	time.Sleep(time.Duration(MetricPartitioningRoutineDelayFirstRunSeconds) * time.Second)

// 	expectedPartitions, err := partitionsForPeriod(time.Now(), int(configuredPrefillDays))

// 	assert.Nil(t, err)

// 	// execute
// 	databaseStatus, err := proxyServiceClient.GetDatabaseStatus(testCtx)

// 	// assert
// 	assert.Nil(t, err)
// 	assert.GreaterOrEqual(t, databaseStatus.TotalProxiedRequestMetricPartitions, configuredPrefillDays)
// 	assert.Equal(t, expectedPartitions[len(expectedPartitions)-1].TableName, databaseStatus.LatestProxiedRequestMetricPartitionTableName)
// }

func TestUnitTestpartitionsForPeriodReturnsErrWhenTooManyPrefillDays(t *testing.T) {
	// prepare
	daysToPrefill := config.MaxMetricPartitioningPrefillPeriodDays + 1

	// execute
	_, err := partitionsForPeriod(time.Now(), daysToPrefill)

	// assert
	assert.NotNil(t, err)
}

func TestUnitTestpartitionsForPeriodReturnsExpectedNumPartitionsWhenPrefillPeriodIsContainedInCurrentMonth(t *testing.T) {
	// prepare

	// pick a date in the middle of a month
	startFrom := time.Date(1989, 5, 11, 12, 0, 0, 0, time.UTC)

	// set prefill period to less then days remaining in month
	// from above date
	daysToPrefill := 3

	// execute
	actualPartitionsForPeriod, err := partitionsForPeriod(startFrom, daysToPrefill)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, daysToPrefill, len(actualPartitionsForPeriod))
}

func TestUnitTestpartitionsForPeriodReturnsExpectedNumPartitionsWhenPrefillPeriodIsNotContainedInCurrentMonth(t *testing.T) {
	// prepare

	// pick a date in the middle of a month
	startFrom := time.Date(1989, 5, 20, 12, 0, 0, 0, time.UTC)

	// set prefill period to more then days remaining in month
	// from above date
	daysToPrefill := 21

	// execute
	actualPartitionsForPeriod, err := partitionsForPeriod(startFrom, daysToPrefill)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, daysToPrefill, len(actualPartitionsForPeriod))
}
