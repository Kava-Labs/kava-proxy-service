// package routines provides configuration and logic
// for running background routines such as metric compaction
// for aggregating and pruning proxied request metrics
package routines

import (
	"fmt"
	"time"

	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// MetricCompactionRoutineConfig wraps values used
// for creating a new metric compaction routine
type MetricCompactionRoutineConfig struct {
	Interval time.Duration
	Database *database.PostgresClient
	Logger   logging.ServiceLogger
}

// MetricCompactionRoutine can be used to
// run a background routine on a configurable interval
// to aggregate and prune historical request metrics
type MetricCompactionRoutine struct {
	interval time.Duration
	*database.PostgresClient
	logging.ServiceLogger
}

// Run runs the metric compaction routine for aggregating
// and pruning historical request metrics, returning error (if any)
// from starting the routine and an error channel which any errors
// encountered during running will be sent on
func (mcr *MetricCompactionRoutine) Run() (<-chan error, error) {
	errorChannel := make(chan error)

	timer := time.Tick(mcr.interval)

	go func() {
		for tick := range timer {
			mcr.Debug().Msg(fmt.Sprintf("tick at %+v", tick))
		}
	}()

	return errorChannel, nil
}

// NewMetricCompactionRoutine creates a new metric compaction routine
// using the provided config, returning the routine and error (if any)
func NewMetricCompactionRoutine(config MetricCompactionRoutineConfig) (*MetricCompactionRoutine, error) {
	return &MetricCompactionRoutine{
		interval:       config.Interval,
		PostgresClient: config.Database,
		ServiceLogger:  config.Logger,
	}, nil
}
