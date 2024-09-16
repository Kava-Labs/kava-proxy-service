package postgres

import (
	"time"

	"github.com/kava-labs/kava-proxy-service/clients/database"
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
	Hostname                    string
	RequestIP                   string `bun:"request_ip"`
	RequestTime                 time.Time
	UserAgent                   *string
	Referer                     *string
	Origin                      *string
	ResponseBackend             string
	ResponseBackendRoute        string
	CacheHit                    bool
	PartOfBatch                 bool
}

func (prm *ProxiedRequestMetric) ToProxiedRequestMetric() *database.ProxiedRequestMetric {
	return &database.ProxiedRequestMetric{
		ID:                          prm.ID,
		MethodName:                  prm.MethodName,
		BlockNumber:                 prm.BlockNumber,
		ResponseLatencyMilliseconds: prm.ResponseLatencyMilliseconds,
		Hostname:                    prm.Hostname,
		RequestIP:                   prm.RequestIP,
		RequestTime:                 prm.RequestTime,
		UserAgent:                   prm.UserAgent,
		Referer:                     prm.Referer,
		Origin:                      prm.Origin,
		ResponseBackend:             prm.ResponseBackend,
		ResponseBackendRoute:        prm.ResponseBackendRoute,
		CacheHit:                    prm.CacheHit,
		PartOfBatch:                 prm.PartOfBatch,
	}
}

func convertProxiedRequestMetric(metric *database.ProxiedRequestMetric) *ProxiedRequestMetric {
	return &ProxiedRequestMetric{
		ID:                          metric.ID,
		MethodName:                  metric.MethodName,
		BlockNumber:                 metric.BlockNumber,
		ResponseLatencyMilliseconds: metric.ResponseLatencyMilliseconds,
		Hostname:                    metric.Hostname,
		RequestIP:                   metric.RequestIP,
		RequestTime:                 metric.RequestTime,
		UserAgent:                   metric.UserAgent,
		Referer:                     metric.Referer,
		Origin:                      metric.Origin,
		ResponseBackend:             metric.ResponseBackend,
		ResponseBackendRoute:        metric.ResponseBackendRoute,
		CacheHit:                    metric.CacheHit,
		PartOfBatch:                 metric.PartOfBatch,
	}
}
