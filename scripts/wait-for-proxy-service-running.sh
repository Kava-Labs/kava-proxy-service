#!/bin/bash
set -x

until curl -f http://localhost:"${PROXY_CONTAINER_PORT}/healthcheck"
do
    echo "waiting for proxy service to be running"
    sleep 0.5
done

echo "proxy service is running"
