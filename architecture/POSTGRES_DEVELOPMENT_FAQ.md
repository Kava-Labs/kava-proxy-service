# Postgres Development FAQs

## Whats the list and status of active database processes?

Run below query to get information about active processes (e.g. queries)

```sql
SELECT
  pid,
  now() - pg_stat_activity.query_start AS duration,
  query,
  state,
  wait_event,
  wait_event_type
FROM pg_stat_activity
WHERE (now() - pg_stat_activity.query_start) > interval '5 seconds';
```

A full list of details retrievable per process is contained [here](https://www.postgresql.org/docs/current/monitoring-stats.html#MONITORING-PG-STAT-ACTIVITY-VIEW)

## Reference Links

- https://aloksinhanov.medium.com/query-vs-exec-vs-prepare-in-golang-e7c49212c36c
- https://go.dev/doc/database/manage-connections#connection_pool_properties
- https://go.dev/doc/database/execute-transactions
- https://medium.com/little-programming-joys/finding-and-killing-long-running-queries-on-postgres-7c4f0449e86d
