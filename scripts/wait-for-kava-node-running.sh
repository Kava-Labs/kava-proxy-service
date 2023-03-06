#!/bin/bash
until docker-compose exec proxy kava status -n tcp://kava:26657
do
    echo "waiting for kava service to be running"
    sleep 1
done

echo "kava service is running"
