package routines

import (
	"testing"
	"time"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/stretchr/testify/assert"
)

func TestE2ETestPartitioningRoutineRunsOnConfiguredInterval(t *testing.T) {
}

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
