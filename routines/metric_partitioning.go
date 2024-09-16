package routines

import (
	"fmt"
	"github.com/kava-labs/kava-proxy-service/clients/database"
	"time"

	"github.com/google/uuid"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// MetricPartitioningRoutineConfig wraps values used
// for creating a new metric partitioning routine
type MetricPartitioningRoutineConfig struct {
	Interval          time.Duration
	DelayFirstRun     time.Duration
	PrefillPeriodDays int
	Database          database.MetricsDatabase
	Logger            logging.ServiceLogger
}

// MetricPartitioningRoutine can be used to
// run a background routine on a configurable interval
// to aggregate and prune historical request metrics
type MetricPartitioningRoutine struct {
	id                string
	interval          time.Duration
	delayFirstRun     time.Duration
	prefillPeriodDays int
	db                database.MetricsDatabase
	logging.ServiceLogger
}

// Run runs the metric partitioning routine for aggregating
// and pruning historical request metrics, returning error (if any)
// from starting the routine and an error channel which any errors
// encountered during running will be sent on
func (mcr *MetricPartitioningRoutine) Run() (<-chan error, error) {
	// do first run
	errorChannel := make(chan error)

	time.Sleep(mcr.delayFirstRun)

	err := mcr.db.Partition(mcr.prefillPeriodDays)
	if err != nil {
		errorChannel <- err
	}

	// do subsequent runs every configured interval
	timer := time.Tick(mcr.interval)

	go func() {
		for tick := range timer {
			mcr.Trace().Msg(fmt.Sprintf("%s tick at %+v", mcr.id, tick))

			err := mcr.db.Partition(mcr.prefillPeriodDays)

			if err != nil {
				errorChannel <- err
			}
		}
	}()

	return errorChannel, nil
}

// NewMetricPartitioningRoutine creates a new metric partitioning routine
// using the provided config, returning the routine and error (if any)
func NewMetricPartitioningRoutine(config MetricPartitioningRoutineConfig) (*MetricPartitioningRoutine, error) {
	return &MetricPartitioningRoutine{
		id:                uuid.New().String(),
		interval:          config.Interval,
		delayFirstRun:     config.DelayFirstRun,
		prefillPeriodDays: config.PrefillPeriodDays,
		db:                config.Database,
		ServiceLogger:     config.Logger,
	}, nil
}
