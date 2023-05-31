package routines

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// MetricPartitioningRoutineConfig wraps values used
// for creating a new metric partitioning routine
type MetricPartitioningRoutineConfig struct {
	Interval          time.Duration
	PrefillPeriodDays int
	Database          *database.PostgresClient
	Logger            logging.ServiceLogger
}

// MetricPartitioningRoutine can be used to
// run a background routine on a configurable interval
// to aggregate and prune historical request metrics
type MetricPartitioningRoutine struct {
	id                string
	interval          time.Duration
	prefillPeriodDays int
	*database.PostgresClient
	logging.ServiceLogger
}

// Run runs the metric partitioning routine for aggregating
// and pruning historical request metrics, returning error (if any)
// from starting the routine and an error channel which any errors
// encountered during running will be sent on
func (mcr *MetricPartitioningRoutine) Run() (<-chan error, error) {
	errorChannel := make(chan error)

	timer := time.Tick(mcr.interval)

	go func() {
		for tick := range timer {
			mcr.Trace().Msg(fmt.Sprintf("%s tick at %+v", mcr.id, tick))
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
		prefillPeriodDays: config.PrefillPeriodDays,
		PostgresClient:    config.Database,
		ServiceLogger:     config.Logger,
	}, nil
}

func Part() {
	// check if now is the time to create new partitions
	// get current days in month
	// https://stackoverflow.com/questions/73880828/list-the-number-of-days-in-current-date-month

	// create partition for each of those days
	// and then attach partition to main table
	// https://www.postgresql.org/docs/current/ddl-partitioning.html
	// create on side and then attach vs create as partition
	// to avoid taking an exclusive lock on the whole table vs just the rows we are adding
	// 	CREATE TABLE IF NOT EXISTS proxied_request_metrics_year2023month6_day01
	//     (LIKE proxied_request_metrics INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
	// ALTER TABLE proxied_request_metrics ATTACH PARTITION proxied_request_metrics_year2023month6_day01
	//     FOR VALUES FROM ('2023-06-01 00:0:0.0') TO ('2023-06-02 00:0:0.0');
	// ignore errors
	// ERROR:  "proxied_request_metrics_year2023month6_day03" is already a partition
	// using raw database client
	// https://bun.uptrace.dev/guide/queries.html#scan-and-exec
	// https://go.dev/doc/database/execute-transactions
	// https://pkg.go.dev/github.com/uptrace/bun#Tx

	// sleep duration
}
