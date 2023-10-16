-- add response backend column, backfilling with "DEFAULT" (the only value to exist up until now)
-- new metrics that omit its value are assumed to have been routed to "DEFAULT" backend
-- also add response backend route. this is the backend url the request was routed to.
ALTER TABLE
  IF EXISTS proxied_request_metrics
ADD
  response_backend character varying DEFAULT 'DEFAULT',
ADD
  response_backend_route character varying;
