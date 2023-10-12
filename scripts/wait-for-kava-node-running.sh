#!/bin/bash
set -x

until kava status -n tcp://localhost:"${KAVA_PRUNING_HOST_COSMOS_RPC_PORT}" | jq; do
    echo "waiting for kava service to be running"
    sleep 1
done

BLOCK_NUMBER=$(kava status -n tcp://localhost:"${KAVA_PRUNING_HOST_COSMOS_RPC_PORT}" | jq .sync_info.latest_block_height)

# seem to need 2 blocks for the evm api to be really "ready"
# stopping at block 1 leads to 504 gateway timeouts when running
# e2e tests after a full restart of all containers from zero state
until [ "$BLOCK_NUMBER" != '"1"' ] && [ "$BLOCK_NUMBER" != '"0"' ] && [ "$BLOCK_NUMBER" != "" ]; do
    BLOCK_NUMBER=$(kava status -n tcp://localhost:"${KAVA_PRUNING_HOST_COSMOS_RPC_PORT}" | jq .sync_info.latest_block_height)
    echo "waiting for kava service to make a block"
    sleep 0.5
done

echo "kava service is running"
