-- Add table for per proxyied request metrics
CREATE TABLE IF NOT EXISTS proxied_request_metrics (
  id SERIAL,
  method_name TEXT NOT NULL,
  block_number BIGINT,
  response_latency_milliseconds NUMERIC,
  request_time timestamptz NOT NULL
);
