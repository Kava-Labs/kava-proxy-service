package database

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// ProxiedRequestMetric contains request metrics for
// a single request proxied by the proxy service
type ProxiedRequestMetric struct {
	bun.BaseModel `bun:"table:proxied_request_metrics,alias:prm"`

	ID                          int64 `bun:",pk,autoincrement"`
	MethodName                  string
	BlockNumber                 *int64
	ResponseLatencyMilliseconds int64
	RequestTime                 time.Time
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
