# Proxied Request Metrics Database Partitioning

Each request to a Kava Labs operated EVM API endpoint results in a new row created in the request metrics table. For mainnet there are currently ~20-30 million EVM requests daily.

For periods of a month or longer, this large amount of data with default Postgres settings is very slow to query and work over, which is a performance bottleneck for both the daily [metric compaction routines](./METRIC_COMPACTION_ROUTINE.md) and [ad hoc queries](https://kava-labs.atlassian.net/wiki/spaces/ENG/pages/1242398721/Useful+Analytic+Queries) made by empowered operators for analytical and operational purposes.

Thankfully, we can leverage [Postgres partitioning features](https://www.postgresql.org/docs/15/ddl-partitioning.html) to optimize for the above two use cases.
> Partitioning refers to splitting what is logically one large table into smaller physical pieces. Partitioning can provide several benefits:

> Query performance can be improved dramatically in certain situations, particularly when most of the heavily accessed rows of the table are in a single partition or a small number of partitions. Partitioning effectively substitutes for the upper tree levels of indexes, making it more likely that the heavily-used parts of the indexes fit in memory.

> When queries or updates access a large percentage of a single partition, performance can be improved by using a sequential scan of that partition instead of using an index, which would require random-access reads scattered across the whole table.

# Proxied Request Metrics Table Partitioning Strategy

The data access pattern for the daily metric compaction routines and ad hoc analytical queries is / can be composed of looking at proxied requests that occur with a 24 hour period, e.g.

1. an on-call engineer who wants to know the distribution of requests by IP address over the last 4 hours
1. metric compaction routines that calculate the daily min, max, average, p50,p90, and p99 request latencies for each API method
1. marketing and product owners who want to know the usage of a set of APIs ver a 3 month period (which can be calculated by concurrently querying for and then serially roll up those totals on a per day basis)

As such, for the `proxied_request_metrics` table, partitions are created spanning a single day of data (with the exception of the initial partition for data created before partitioning was implemented), and using the `request_time` timestamp value as the range key

as shown conceptually below

![Metric Partitioning Conceptual Overview](./images/metric_partitioning_conceptual.jpg)

or concretely from the perspective of the database

```text
# connect to database locally
$ make reset ready debug-database

postgres=# \d+ proxied_request_metrics;
 Partitioned table "public.proxied_request_metrics"
...omitted schema...
 Partition key: RANGE (request_time)
 Indexes:
    "block_number_idx" btree (block_number)
    "hostname_idx" btree (hostname)
    "id_idx" btree (id)
    "method_name_idx" btree (method_name)
    "request_time_idx" btree (request_time)
 Partitions: proxied_request_metrics_year2023month1_day1 FOR VALUES FROM ('2023-01-01 00:00:00') TO ('2023-05-04 00:00:00'),
            proxied_request_metrics_year2023month5_day10 FOR VALUES FROM ('2023-05-10 00:00:00') TO ('2023-05-11 00:00:00'),
            proxied_request_metrics_year2023month5_day11 FOR VALUES FROM ('2023-05-11 00:00:00') TO ('2023-05-12 00:00:00'),
...moar partitions...
```

# Partitioning Routines

*Coming Soon*

Partitions are great, but partitioning requires continuous maintenance.

From the [Postgres docs](https://www.postgresql.org/docs/current/ddl-partitioning.html)

```text
Inserting data into the parent table that does not map to one of the existing partitions will cause an error; an appropriate partition must be added manually.
```

## Monitoring Status of Partitioning

*Coming Soon*

`/status/database`

ops, ci and local tests need partitions for the current week and to be able to query the status of those processes for

```text
migration_status:enum
last_pruned:timestamp
pruning_status:enum
partitioning_status:enum
rolling_error_rate:float
rolling_error_count:int
```
