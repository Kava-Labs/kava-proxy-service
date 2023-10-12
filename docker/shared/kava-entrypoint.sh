#!/bin/bash

# log all commands to stdout and stop the script on the first error
set -ex

SHARED_GENTX_DIR=/docker/shared/gentx
NUM_VALIDATORS=$(cat /docker/shared/NUM_VALIDATORS)

SKIP_SETUP=false

# exit early if geneis.json already exists
# which will happen if the kava docker container is stopped and later restarted
if test -f "/root/.kava/config/genesis.json"; then
    SKIP_SETUP=true
    echo "genesis.json alredy exists, skipping chain init and validator initilization"
else
    # create default genesis and node config
    kava init test --chain-id=localnet_7777-1

    # copy over temporary shared genesis
    cp /docker/shared/genesis.json /root/.kava/config/genesis.json
    # # ensure evm api listens on all addresses
    sed -i 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/g' /root/.kava/config/app.toml

    # use the test backend to avoid prompts when storing and accessing keys
    kava config keyring-backend test

    # create an account for the delegator
    kava keys add kava-localnet-delegator

    MY_ADDRESS=$(kava keys show kava-localnet-delegator -a)
    MY_NODE_ID=$(kava tendermint show-node-id)

    # # symlink gentx dir to directory shared between containers
    # ln -s /docker/shared/gentx /root/.kava/config/gentx

    # add the delegator account to the default genesis (required for gentx)
    kava add-genesis-account "$MY_ADDRESS" 1000000000stake

    # create genesis info for a validator staked by the delegator above
    kava gentx kava-localnet-delegator 500000000stake \
        --chain-id=localnet_7777-1 \
        --moniker="kava-localnet-validator"

    # share this validator's gentx with the other validators
    cp /root/.kava/config/gentx/*.json "$SHARED_GENTX_DIR/$MY_ADDRESS-$MY_NODE_ID@$CONTAINER_NAME"

    # wait for correct number of gentx's in folder
    while true; do
        current_file_count=$(find "$SHARED_GENTX_DIR" -maxdepth 1 -type f | wc -l)
        if [ "$current_file_count" -ge "$NUM_VALIDATORS" ]; then
            echo "Folder now contains $current_file_count files."
            break
        else
            echo "Waiting $NUM_VALIDATORS genesis txs. Current count: $current_file_count"
            sleep 0.25
        fi
    done

    # reset genesis. genesis accounts will be added in correct order in setup below
    # adding them in a different order in each different validator results in an AppHash mismatch.
    cp /docker/shared/genesis.json /root/.kava/config/genesis.json
fi

MY_ADDRESS=$(kava keys show kava-localnet-delegator -a)
MY_NODE_ID=$(kava tendermint show-node-id)

# discover all peers from the shared gentx dir.
# if we are in setup phase, create genesis accounts for them.
PEERS=()
for file in "$SHARED_GENTX_DIR"/*; do
    # shared format is <ADDRESS>-<NODE_ID>@HOST
    # separate each of those pieces
    IFS='-' read -ra parts <<<"${file#"$SHARED_GENTX_DIR/"}"
    ADDRESS="${parts[0]}"
    VAL_HOST="${parts[1]}"
    IFS='@' read -ra parts <<<"$VAL_HOST"
    NODE_ID="${parts[0]}"
    VAL_HOST="${parts[1]}"

    PEER="$NODE_ID@$VAL_HOST:26656"
    echo "$file -> address: $ADDRESS; node id: $NODE_ID; host: $VAL_HOST ($PEER)"

    # create genesis account & copy gentx if doing initial setup
    # all validators will add accounts in same order so result genesis should be the same.
    if [ "$SKIP_SETUP" == false ]; then
        cp "$file" "/root/.kava/config/gentx/gentx-$NODE_ID.json"
        kava add-genesis-account "$ADDRESS" 1000000000stake
    fi

    # add to peer list, skip adding self.
    if [ "$ADDRESS" != "$MY_ADDRESS" ]; then
        PEERS+=("$PEER")
        continue
    fi
done

if [ "$SKIP_SETUP" == false ]; then
    # merge all gentxs with previously generated default genesis
    kava collect-gentxs
fi

# set config for kava processes to use
cp /docker/kava/config.toml ~/.kava/config/config.toml

# start the kava process with all peers
COMMA_DELIMITED_PEERS=$(
    IFS=,
    echo "${PEERS[*]}"
)
kava start --p2p.persistent_peers "$COMMA_DELIMITED_PEERS"

# run forever (kava start is non-blocking)
tail -f /dev/null
