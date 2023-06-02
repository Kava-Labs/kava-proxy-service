package service

// DatabaseStatusResponse wraps values
// returned by calls to /status/database
type DatabaseStatusResponse struct {
	LatestProxiedRequestMetricPartitionTableName string `json:"latest_proxied_request_metric_partition_table_name"` // name of the latest created and currently attached partition for the proxied_request_metrics table
	TotalProxiedRequestMetricPartitions          int64  `json:"total_proxied_request_metric_partitions"`            // total number of attached partitions for the proxied_request_metrics table
}
