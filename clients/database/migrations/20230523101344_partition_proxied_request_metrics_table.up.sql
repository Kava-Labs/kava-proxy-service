-- rename current table to preserve it's data
ALTER TABLE proxied_request_metrics rename to proxied_request_metrics_old;

-- create new version of table with timestamp range based partitioning
CREATE TABLE proxied_request_metrics(
  id bigserial, -- analytic tables usually hit the 4 byte limit
  method_name character varying not null, -- same as text but often used to signify short strings to people and code generators reading the schema
  block_number bigint,
  response_latency_milliseconds int,
  request_time timestamp without time zone NOT NULL,
  request_ip character varying,
  hostname character varying,
  user_agent character varying,
  referer character varying,
  origin character varying
) PARTITION BY RANGE (request_time);

-- Create partitions for the previous data collection periods

-- partition of requests that occurred more than 2 weeks before the kava 13 network upgrade
CREATE TABLE proxied_request_metrics_year2023month1_day1 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-01-01 00:0:0.0') TO ('2023-05-03 23:59:59.999999');

-- create daily partitions for all requests that occured from within 2 weeks of the kava 13 network upgrade or 2 weeks after the upgrade
CREATE TABLE proxied_request_metrics_year2023month5_day4 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-04 00:0:0.0') TO ('2023-05-04 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day5 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-05 00:0:0.0') TO ('2023-05-05 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day6 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-06 00:0:0.0') TO ('2023-05-06 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day7 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-07 00:0:0.0') TO ('2023-05-07 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day8 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-08 00:0:0.0') TO ('2023-05-08 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day9 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-09 00:0:0.0') TO ('2023-05-09 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day10 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-10 00:0:0.0') TO ('2023-05-10 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day11 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-11 00:0:0.0') TO ('2023-05-11 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day12 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-12 00:0:0.0') TO ('2023-05-12 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day13 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-13 00:0:0.0') TO ('2023-05-13 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day14 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-14 00:0:0.0') TO ('2023-05-14 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day15 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-15 00:0:0.0') TO ('2023-05-15 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day16 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-16 00:0:0.0') TO ('2023-05-16 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day17 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-17 00:0:0.0') TO ('2023-05-17 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day18 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-18 00:0:0.0') TO ('2023-05-18 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day19 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-19 00:0:0.0') TO ('2023-05-19 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day20 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-20 00:0:0.0') TO ('2023-05-20 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day21 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-21 00:0:0.0') TO ('2023-05-21 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day22 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-22 00:0:0.0') TO ('2023-05-22 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day23 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-23 00:0:0.0') TO ('2023-05-23 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day24 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-24 00:0:0.0') TO ('2023-05-24 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day25 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-25 00:0:0.0') TO ('2023-05-25 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day26 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-26 00:0:0.0') TO ('2023-05-26 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day27 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-27 00:0:0.0') TO ('2023-05-27 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day28 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-28 00:0:0.0') TO ('2023-05-28 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day29 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-29 00:0:0.0') TO ('2023-05-29 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day30 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-30 00:0:0.0') TO ('2023-05-30 23:59:59.999999');
    CREATE TABLE proxied_request_metrics_year2023month5_day31 PARTITION OF proxied_request_metrics
    FOR VALUES FROM ('2023-05-31 00:0:0.0') TO ('2023-05-31 23:59:59.999999');
-- new partitions will be auto-created by the metric compaction routines

-- copy the data to our new partitioned table
INSERT INTO proxied_request_metrics (method_name,block_number,response_latency_milliseconds,request_time,request_ip,hostname,user_agent,referer,origin) SELECT method_name,block_number,response_latency_milliseconds,request_time,request_ip,hostname,user_agent,referer,origin FROM proxied_request_metrics_old;

-- get rid of the old table
DROP TABLE proxied_request_metrics_old;

-- set up previous indices
CREATE INDEX  IF NOT EXISTS method_name_idx ON proxied_request_metrics(method_name);
CREATE INDEX  IF NOT EXISTS block_number_idx ON proxied_request_metrics(block_number);
CREATE INDEX  IF NOT EXISTS hostname_idx ON proxied_request_metrics(hostname);
CREATE INDEX  IF NOT EXISTS request_time_idx ON proxied_request_metrics(request_time);
CREATE INDEX  IF NOT EXISTS id_idx ON proxied_request_metrics(id);
