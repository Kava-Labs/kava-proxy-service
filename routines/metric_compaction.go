// package routines provides configuration and logic
// for running background routines such as metric compaction
// for aggregating and pruning proxied request metrics
package routines

import (
	"fmt"
	"github.com/kava-labs/kava-proxy-service/clients/database"
	"time"

	"github.com/google/uuid"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// MetricCompactionRoutineConfig wraps values used
// for creating a new metric compaction routine
type MetricCompactionRoutineConfig struct {
	Interval time.Duration
	Database database.MetricsDatabase
	Logger   logging.ServiceLogger
}

// MetricCompactionRoutine can be used to
// run a background routine on a configurable interval
// to aggregate and prune historical request metrics
type MetricCompactionRoutine struct {
	id       string
	interval time.Duration
	db       database.MetricsDatabase
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
			mcr.Trace().Msg(fmt.Sprintf("%s tick at %+v", mcr.id, tick))
		}
	}()

	return errorChannel, nil
}

// NewMetricCompactionRoutine creates a new metric compaction routine
// using the provided config, returning the routine and error (if any)
func NewMetricCompactionRoutine(config MetricCompactionRoutineConfig) (*MetricCompactionRoutine, error) {
	return &MetricCompactionRoutine{
		id:            uuid.New().String(),
		interval:      config.Interval,
		db:            config.Database,
		ServiceLogger: config.Logger,
	}, nil
}
