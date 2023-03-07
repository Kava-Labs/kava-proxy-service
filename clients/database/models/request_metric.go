package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ProxiedRequestMetric contains request metrics for
// a single request proxied by the proxy service
type ProxiedRequestMetric struct {
	bun.BaseModel `bun:"table:proxied_request_metrics,alias:prm"`

	ID                          int64 `bun:",pk,autoincrement"`
	MethodName                  string
	BlockNumber                 int64
	ResponseLatencyMilliseconds float64
	RequestTime                 time.Time
}
