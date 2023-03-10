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
