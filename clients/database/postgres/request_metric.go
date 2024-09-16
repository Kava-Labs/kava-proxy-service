package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/kava-labs/kava-proxy-service/clients/database"
)

const (
	ProxiedRequestMetricsTableName = "proxied_request_metrics"
)

// Save saves the current ProxiedRequestMetric to
// the database, returning error (if any).
// If db is nil, returns nil error.
func (c *Client) SaveProxiedRequestMetric(ctx context.Context, metric *database.ProxiedRequestMetric) error {
	prm := convertProxiedRequestMetric(metric)
	_, err := c.db.NewInsert().Model(prm).Exec(ctx)

	return err
}

// ListProxiedRequestMetricsWithPagination returns a page of max
// `limit` ProxiedRequestMetrics from the offset specified by`cursor`
// error (if any) along with a cursor to use to fetch the next page
// if the cursor is 0 no more pages exists.
// Uses only in tests. If db is nil, returns empty slice and 0 cursor.
func (c *Client) ListProxiedRequestMetricsWithPagination(ctx context.Context, cursor int64, limit int) ([]*database.ProxiedRequestMetric, int64, error) {
	var proxiedRequestMetrics []ProxiedRequestMetric
	var nextCursor int64

	count, err := c.db.NewSelect().Model(&proxiedRequestMetrics).Where("ID > ?", cursor).Limit(limit).ScanAndCount(ctx)

	// look up the id of the last
	if count == limit {
		nextCursor = proxiedRequestMetrics[count-1].ID
	}

	metrics := make([]*database.ProxiedRequestMetric, 0, len(proxiedRequestMetrics))
	for _, metric := range proxiedRequestMetrics {
		metrics = append(metrics, metric.ToProxiedRequestMetric())
	}

	// otherwise leave nextCursor as 0 to signal no more rows
	return metrics, nextCursor, err
}

// CountAttachedProxiedRequestMetricPartitions returns the current
// count of attached partitions for the ProxiedRequestMetricsTableName
// and error (if any).
// If db is nil, returns 0 and nil error.
func (c *Client) CountAttachedProxiedRequestMetricPartitions(ctx context.Context) (int64, error) {
	var count int64

	countPartitionsQuery := fmt.Sprintf(`
	SELECT COUNT (*)
	FROM pg_inherits
		JOIN pg_class parent            ON pg_inherits.inhparent = parent.oid
		JOIN pg_class child             ON pg_inherits.inhrelid   = child.oid
		JOIN pg_namespace nmsp_parent   ON nmsp_parent.oid  = parent.relnamespace
		JOIN pg_namespace nmsp_child    ON nmsp_child.oid   = child.relnamespace
	WHERE parent.relname='%s';`, ProxiedRequestMetricsTableName)

	row := c.db.QueryRow(countPartitionsQuery)
	err := row.Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("error %s querying %s count of partitions", err, countPartitionsQuery)

	}

	return count, nil
}

// GetLastCreatedAttachedProxiedRequestMetricsPartitionName gets the table name
// for the last created (and attached) proxied request metrics partition
// Used for status check. If db is nil, returns empty string and nil error.
func (c *Client) GetLastCreatedAttachedProxiedRequestMetricsPartitionName(ctx context.Context) (string, error) {
	var lastCreatedAttachedPartitionName string

	lastCreatedAttachedPartitionNameQuery := fmt.Sprintf(`
SELECT
	child.relname       AS child
FROM pg_inherits
	JOIN pg_class parent            ON pg_inherits.inhparent = parent.oid
	JOIN pg_class child             ON pg_inherits.inhrelid   = child.oid
	JOIN pg_namespace nmsp_parent   ON nmsp_parent.oid  = parent.relnamespace
	JOIN pg_namespace nmsp_child    ON nmsp_child.oid   = child.relnamespace
WHERE parent.relname='%s' order by child.oid desc limit 1;`, ProxiedRequestMetricsTableName)

	row := c.db.QueryRow(lastCreatedAttachedPartitionNameQuery)
	err := row.Scan(&lastCreatedAttachedPartitionName)

	if err != nil {
		return lastCreatedAttachedPartitionName, fmt.Errorf("error %s querying %s latest proxied request metrics partition name", err, lastCreatedAttachedPartitionNameQuery)

	}

	return lastCreatedAttachedPartitionName, nil
}

// DeleteProxiedRequestMetricsOlderThanNDays deletes
// all proxied request metrics older than the specified
// days, returning error (if any).
// Used during pruning process. If db is nil, returns nil error.
func (c *Client) DeleteProxiedRequestMetricsOlderThanNDays(ctx context.Context, n int64) error {
	_, err := c.db.NewDelete().Model((*ProxiedRequestMetric)(nil)).Where(fmt.Sprintf("request_time < now() - interval '%d' day", n)).Exec(ctx)

	return err
}

// Exec is not part of database.MetricsDatabase interface, so it is used only in the implementation for test purposes.
func (c *Client) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.db.Exec(query, args...)
}
