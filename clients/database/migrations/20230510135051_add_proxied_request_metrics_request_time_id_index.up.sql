CREATE INDEX CONCURRENTLY IF NOT EXISTS request_time_idx ON proxied_request_metrics(request_time);
CREATE INDEX CONCURRENTLY IF NOT EXISTS id_idx ON proxied_request_metrics(id);
