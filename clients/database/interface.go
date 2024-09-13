package database

import (
	"context"
	"time"
)

// MetricsDatabase is an interface for interacting with the database
type MetricsDatabase interface {
	SaveProxiedRequestMetric(ctx context.Context, metric *ProxiedRequestMetric) error
	ListProxiedRequestMetricsWithPagination(ctx context.Context, cursor int64, limit int) ([]*ProxiedRequestMetric, int64, error)
	CountAttachedProxiedRequestMetricPartitions(ctx context.Context) (int64, error)
	GetLastCreatedAttachedProxiedRequestMetricsPartitionName(ctx context.Context) (string, error)
	DeleteProxiedRequestMetricsOlderThanNDays(ctx context.Context, n int64) error

	HealthCheck() error
	Partition(prefillPeriodDays int) error
}

type ProxiedRequestMetric struct {
	ID                          int64
	MethodName                  string
	BlockNumber                 *int64
	ResponseLatencyMilliseconds int64
	Hostname                    string
	RequestIP                   string
	RequestTime                 time.Time
	UserAgent                   *string
	Referer                     *string
	Origin                      *string
	ResponseBackend             string
	ResponseBackendRoute        string
	CacheHit                    bool
	PartOfBatch                 bool
}
