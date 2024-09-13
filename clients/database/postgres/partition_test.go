package postgres

import (
	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestUnitTestPartitionsForPeriodReturnsExpectedNumPartitionsWhenPrefillPeriodIsNotContainedInCurrentMonth(t *testing.T) {
	// prepare

	// pick a date in the middle of a month
	startFrom := time.Date(1989, 5, 20, 12, 0, 0, 0, time.UTC)

	// set prefill period to more then days remaining in month
	// from above date
	daysToPrefill := 21

	// execute
	actualPartitionsForPeriod, err := PartitionsForPeriod(startFrom, daysToPrefill)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, daysToPrefill, len(actualPartitionsForPeriod))
}

func TestUnitTestPartitionsForPeriodReturnsErrWhenTooManyPrefillDays(t *testing.T) {
	// prepare
	daysToPrefill := config.MaxMetricPartitioningPrefillPeriodDays + 1

	// execute
	_, err := PartitionsForPeriod(time.Now(), daysToPrefill)

	// assert
	assert.NotNil(t, err)
}

func TestUnitTestPartitionsForPeriodReturnsExpectedNumPartitionsWhenPrefillPeriodIsContainedInCurrentMonth(t *testing.T) {
	// prepare

	// pick a date in the middle of a month
	startFrom := time.Date(1989, 5, 11, 12, 0, 0, 0, time.UTC)

	// set prefill period to less then days remaining in month
	// from above date
	daysToPrefill := 3

	// execute
	actualPartitionsForPeriod, err := PartitionsForPeriod(startFrom, daysToPrefill)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, daysToPrefill, len(actualPartitionsForPeriod))
}
