// package routines provides configuration and logic
// for running background routines such as metric Pruning
// for aggregating and pruning proxied request metrics
package routines

import (
	"context"
	"fmt"
	"github.com/kava-labs/kava-proxy-service/clients/database"
	"time"

	"github.com/google/uuid"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// MetricPruningRoutineConfig wraps values used
// for creating a new metric Pruning routine
type MetricPruningRoutineConfig struct {
	Interval                     time.Duration
	StartDelay                   time.Duration
	MaxRequestMetricsHistoryDays int64
	Database                     database.MetricsDatabase
	Logger                       logging.ServiceLogger
}

// MetricPruningRoutine can be used to
// run a background routine on a configurable interval
// to aggregate and prune historical request metrics
type MetricPruningRoutine struct {
	id                           string
	interval                     time.Duration
	startDelay                   time.Duration
	maxRequestMetricsHistoryDays int64
	db                           database.MetricsDatabase
	logging.ServiceLogger
}

// Run runs the metric Pruning routine for aggregating
// and pruning historical request metrics, returning error (if any)
// from starting the routine and an error channel which any errors
// encountered during running will be sent on
func (mpr *MetricPruningRoutine) Run() (<-chan error, error) {
	errorChannel := make(chan error)

	time.Sleep(mpr.startDelay)

	timer := time.Tick(mpr.interval)

	go func() {
		for tick := range timer {
			mpr.Trace().Msg(fmt.Sprintf("%s tick at %+v", mpr.id, tick))

			mpr.db.DeleteProxiedRequestMetricsOlderThanNDays(context.Background(), mpr.maxRequestMetricsHistoryDays)
		}
	}()

	return errorChannel, nil
}

// NewMetricPruningRoutine creates a new metric Pruning routine
// using the provided config, returning the routine and error (if any)
func NewMetricPruningRoutine(config MetricPruningRoutineConfig) (*MetricPruningRoutine, error) {
	return &MetricPruningRoutine{
		id:                           uuid.New().String(),
		interval:                     config.Interval,
		startDelay:                   config.StartDelay,
		maxRequestMetricsHistoryDays: config.MaxRequestMetricsHistoryDays,
		db:                           config.Database,
		ServiceLogger:                config.Logger,
	}, nil
}
