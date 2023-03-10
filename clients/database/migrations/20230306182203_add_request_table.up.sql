-- Add table for per proxyied request metrics
CREATE TABLE IF NOT EXISTS proxied_request_metrics (
  id bigserial, -- analytic tables usually hit the 4 byte limit
  method_name character varying not null, -- same as text but often used to signify short strings to people and code generators reading the schema
  block_number bigint,
  response_latency_milliseconds int,
  request_time timestamp without time zone NOT NULL
);
