package database

import "context"

type MetricsDatabase interface {
	SaveProxiedRequestMetric(ctx context.Context, prm *ProxiedRequestMetric) error
	ListProxiedRequestMetricsWithPagination(ctx context.Context, cursor int64, limit int) ([]ProxiedRequestMetric, int64, error)
	CountAttachedProxiedRequestMetricPartitions(ctx context.Context) (int64, error)
	GetLastCreatedAttachedProxiedRequestMetricsPartitionName(ctx context.Context) (string, error)
	DeleteProxiedRequestMetricsOlderThanNDays(ctx context.Context, days int) error
}
