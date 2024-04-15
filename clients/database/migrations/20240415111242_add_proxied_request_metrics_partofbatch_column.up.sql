ALTER TABLE
  IF EXISTS proxied_request_metrics
ADD
  part_of_batch boolean NOT NULL DEFAULT false;
