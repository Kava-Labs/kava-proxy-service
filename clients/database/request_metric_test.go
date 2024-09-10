package database

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNoDatabaseSave(t *testing.T) {
	prm := ProxiedRequestMetric{}
	err := prm.Save(context.Background(), nil)
	require.NoError(t, err)
}

func TestNoDatabaseListProxiedRequestMetricsWithPagination(t *testing.T) {
	proxiedRequestMetrics, cursor, err := ListProxiedRequestMetricsWithPagination(context.Background(), nil, 0, 0)
	require.NoError(t, err)
	require.Empty(t, proxiedRequestMetrics)
	require.Zero(t, cursor)
}

func TestNoDatabaseCountAttachedProxiedRequestMetricPartitions(t *testing.T) {
	count, err := CountAttachedProxiedRequestMetricPartitions(context.Background(), nil)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestGetLastCreatedAttachedProxiedRequestMetricsPartitionName(t *testing.T) {
	partitionName, err := GetLastCreatedAttachedProxiedRequestMetricsPartitionName(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, partitionName)
}

func TestDeleteProxiedRequestMetricsOlderThanNDays(t *testing.T) {
	err := DeleteProxiedRequestMetricsOlderThanNDays(context.Background(), nil, 0)
	require.NoError(t, err)
}
