package noop

import (
	"context"
	"github.com/kava-labs/kava-proxy-service/clients/database"
)

// Noop is a database client that does nothing
type Noop struct{}

func New() *Noop {
	return &Noop{}
}

func (e *Noop) SaveProxiedRequestMetric(ctx context.Context, metric *database.ProxiedRequestMetric) error {
	return nil
}

func (e *Noop) ListProxiedRequestMetricsWithPagination(ctx context.Context, cursor int64, limit int) ([]*database.ProxiedRequestMetric, int64, error) {
	return []*database.ProxiedRequestMetric{}, 0, nil
}

func (e *Noop) CountAttachedProxiedRequestMetricPartitions(ctx context.Context) (int64, error) {
	return 0, nil
}

func (e *Noop) GetLastCreatedAttachedProxiedRequestMetricsPartitionName(ctx context.Context) (string, error) {
	return "", nil
}

func (e *Noop) DeleteProxiedRequestMetricsOlderThanNDays(ctx context.Context, n int64) error {
	return nil
}

func (e *Noop) HealthCheck() error {
	return nil
}

func (e *Noop) Partition(prefillPeriodDays int) error {
	return nil
}
