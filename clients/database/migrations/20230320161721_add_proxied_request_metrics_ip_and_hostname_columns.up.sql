ALTER TABLE IF EXISTS proxied_request_metrics ADD request_ip character varying;

ALTER TABLE IF EXISTS proxied_request_metrics ADD hostname character varying;
