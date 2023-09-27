#!/bin/bash
set -x

while true
do
    CREATED_PARTITIONS=$(curl -f http://localhost:"${PROXY_CONTAINER_PORT}/status/database" | jq '.total_proxied_request_metric_partitions')

    echo "$CREATED_PARTITIONS partitions exist"

    if [ "$CREATED_PARTITIONS" -ge "$MINIMUM_REQUIRED_PARTITIONS" ]
    then
        echo "MINIMUM_REQUIRED_PARTITIONS created $CREATED_PARTITIONS; required: $MINIMUM_REQUIRED_PARTITIONS"
        exit 0
    fi

    sleep 1
done
