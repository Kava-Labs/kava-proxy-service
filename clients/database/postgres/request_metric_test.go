package postgres

import (
	"context"
	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNoDatabaseSave(t *testing.T) {
	db := &Client{}

	prm := &database.ProxiedRequestMetric{}
	err := db.SaveProxiedRequestMetric(context.Background(), prm)
	require.Error(t, err)
}

func TestNoDatabaseListProxiedRequestMetricsWithPagination(t *testing.T) {
	db := &Client{}

	_, _, err := db.ListProxiedRequestMetricsWithPagination(context.Background(), 0, 0)
	require.Error(t, err)
}

func TestNoDatabaseCountAttachedProxiedRequestMetricPartitions(t *testing.T) {
	db := &Client{}

	_, err := db.CountAttachedProxiedRequestMetricPartitions(context.Background())
	require.Error(t, err)
}

func TestGetLastCreatedAttachedProxiedRequestMetricsPartitionName(t *testing.T) {
	db := &Client{}

	_, err := db.GetLastCreatedAttachedProxiedRequestMetricsPartitionName(context.Background())
	require.Error(t, err)
}

func TestDeleteProxiedRequestMetricsOlderThanNDays(t *testing.T) {
	db := &Client{}

	err := db.DeleteProxiedRequestMetricsOlderThanNDays(context.Background(), 0)
	require.Error(t, err)
}
