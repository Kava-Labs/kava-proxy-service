ALTER TABLE IF EXISTS proxied_request_metrics ADD user_agent character varying;

ALTER TABLE IF EXISTS proxied_request_metrics ADD referer character varying;
