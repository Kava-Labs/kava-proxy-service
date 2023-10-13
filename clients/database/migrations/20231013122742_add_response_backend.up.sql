-- add response backend column, backfilling with "DEFAULT" (the only value to exist up until now)
-- new metrics that omit its value are assumed to have been routed to "DEFAULT" backend
ALTER TABLE
  IF EXISTS proxied_request_metrics
ADD
  response_backend character varying DEFAULT 'DEFAULT';
