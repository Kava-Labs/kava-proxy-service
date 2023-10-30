package database

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

const (
	ProxiedRequestMetricsTableName = "proxied_request_metrics"
)

// ProxiedRequestMetric contains request metrics for
// a single request proxied by the proxy service
type ProxiedRequestMetric struct {
	bun.BaseModel `bun:"table:proxied_request_metrics,alias:prm"`

	ID                          int64 `bun:",pk,autoincrement"`
	MethodName                  string
	BlockNumber                 *int64
	ResponseLatencyMilliseconds int64
	Hostname                    string
	RequestIP                   string `bun:"request_ip"`
	RequestTime                 time.Time
	UserAgent                   *string
	Referer                     *string
	Origin                      *string
	ResponseBackend             string
	ResponseBackendRoute        string
	CacheHit                    bool
}

// Save saves the current ProxiedRequestMetric to
// the database, returning error (if any)
func (prm *ProxiedRequestMetric) Save(ctx context.Context, db *bun.DB) error {
	_, err := db.NewInsert().Model(prm).Exec(ctx)

	return err
}

// ListProxiedRequestMetricsWithPagination returns a page of max
// `limit` ProxiedRequestMetrics from the offset specified by`cursor`
// error (if any) along with a cursor to use to fetch the next page
// if the cursor is 0 no more pages exists
func ListProxiedRequestMetricsWithPagination(ctx context.Context, db *bun.DB, cursor int64, limit int) ([]ProxiedRequestMetric, int64, error) {
	var proxiedRequestMetrics []ProxiedRequestMetric
	var nextCursor int64

	count, err := db.NewSelect().Model(&proxiedRequestMetrics).Where("ID > ?", cursor).Limit(limit).ScanAndCount(ctx)

	// look up the id of the last
	if count == limit {
		nextCursor = proxiedRequestMetrics[count-1].ID
	}

	// otherwise leave nextCursor as 0 to signal no more rows
	return proxiedRequestMetrics, nextCursor, err
}

// CountAttachedProxiedRequestMetricPartitions returns the current
// count of attached partitions for the ProxiedRequestMetricsTableName
// and error (if any)
func CountAttachedProxiedRequestMetricPartitions(ctx context.Context, db *bun.DB) (int64, error) {
	var count int64

	countPartitionsQuery := fmt.Sprintf(`
	SELECT COUNT (*)
	FROM pg_inherits
		JOIN pg_class parent            ON pg_inherits.inhparent = parent.oid
		JOIN pg_class child             ON pg_inherits.inhrelid   = child.oid
		JOIN pg_namespace nmsp_parent   ON nmsp_parent.oid  = parent.relnamespace
		JOIN pg_namespace nmsp_child    ON nmsp_child.oid   = child.relnamespace
	WHERE parent.relname='%s';`, ProxiedRequestMetricsTableName)

	row := db.QueryRow(countPartitionsQuery)
	err := row.Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("error %s querying %s count of partitions", err, countPartitionsQuery)

	}

	return count, nil
}

// GetLastCreatedAttachedProxiedRequestMetricsPartitionName gets the table name
// for the last created (and attached) proxied request metrics partition
func GetLastCreatedAttachedProxiedRequestMetricsPartitionName(ctx context.Context, db *bun.DB) (string, error) {
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

	row := db.QueryRow(lastCreatedAttachedPartitionNameQuery)
	err := row.Scan(&lastCreatedAttachedPartitionName)

	if err != nil {
		return lastCreatedAttachedPartitionName, fmt.Errorf("error %s querying %s latest proxied request metrics partition name", err, lastCreatedAttachedPartitionNameQuery)

	}

	return lastCreatedAttachedPartitionName, nil
}

// DeleteProxiedRequestMetricsOlderThanNDays deletes
// all proxied request metrics older than the specified
// days, returning error (if any)
func DeleteProxiedRequestMetricsOlderThanNDays(ctx context.Context, db *bun.DB, n int64) error {
	_, err := db.NewDelete().Model((*ProxiedRequestMetric)(nil)).Where(fmt.Sprintf("request_time < now() - interval '%d' day", n)).Exec(ctx)

	return err
}
