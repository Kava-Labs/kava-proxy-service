#!/bin/bash

# log all commands to stdout and stop the script on the first error
set -ex

SHARED_DIR=/docker/shared

# exit early if geneis.json already exists
# which will happen if the kava docker container is stopped and later restarted
if test -f "/root/.kava/config/genesis.json"; then
  echo "genesis.json alredy exists, skipping chain init and validator initilization"
else
  # create default genesis and node config
  kava init test --chain-id=localnet_7777-1

  # ensure evm api listens on all addresses
  sed -i 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/g' /root/.kava/config/app.toml

  # wait for genesis.json from validator
  while true; do
    current_file_count=$(find "$SHARED_DIR/genesis.json" -maxdepth 1 -type f | wc -l)
    if [ "$current_file_count" == 1 ]; then
      echo "Found shared genesis.json from validator."
      break
    else
      echo "Waiting for validator to share genesis.json."
      sleep 0.25
    fi
  done

  # copy over genesis file
  cp "$SHARED_DIR/genesis.json" /root/.kava/config/genesis.json
fi

# set config for kava processes to use
cp /docker/shared/config.toml ~/.kava/config/config.toml

# get node id of validator
VALIDATOR_NODE_ID="$(cat /docker/shared/VALIDATOR_NODE_ID)"

# start the kava process
kava start --p2p.persistent_peers "$VALIDATOR_NODE_ID@kava-validator:26656"

# run forever (kava start is non-blocking)
tail -f /dev/null
