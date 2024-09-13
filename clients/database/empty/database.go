package empty

import (
	"context"
	"github.com/kava-labs/kava-proxy-service/clients/database"
)

// Empty is a database client that does nothing
type Empty struct{}

func New() *Empty {
	return &Empty{}
}

func (e *Empty) SaveProxiedRequestMetric(ctx context.Context, metric *database.ProxiedRequestMetric) error {
	return nil
}

func (e *Empty) ListProxiedRequestMetricsWithPagination(ctx context.Context, cursor int64, limit int) ([]*database.ProxiedRequestMetric, int64, error) {
	return []*database.ProxiedRequestMetric{}, 0, nil
}

func (e *Empty) CountAttachedProxiedRequestMetricPartitions(ctx context.Context) (int64, error) {
	return 0, nil
}

func (e *Empty) GetLastCreatedAttachedProxiedRequestMetricsPartitionName(ctx context.Context) (string, error) {
	return "", nil
}

func (e *Empty) DeleteProxiedRequestMetricsOlderThanNDays(ctx context.Context, n int64) error {
	return nil
}

func (e *Empty) HealthCheck() error {
	return nil
}

func (e *Empty) Partition(prefillPeriodDays int) error {
	return nil
}
