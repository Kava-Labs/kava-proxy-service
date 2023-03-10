CREATE INDEX IF NOT EXISTS method_name_idx ON proxied_request_metrics(method_name);

--migration:split

CREATE INDEX IF NOT EXISTS block_number_idx ON proxied_request_metrics(block_number);
