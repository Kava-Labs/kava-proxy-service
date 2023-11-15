#!/bin/bash

# log all commands to stdout and stop the script on the first error
set -ex

evmFaucetMnemonic='canvas category slow immune screen van spirit ring blossom vanish mail pencil resource scan razor online gap void time marine topic swarm exhaust oak'
# Private Key in hex: 296da4e8defa5691077b310e10f0ed0ee4993e6418a0df86b155be5d24ae1b7c
# EVM Address in hex: 0x661C3ECC5bf3cdB64FC14c9fE9Fb64a21D24c51c

# exit early if geneis.json already exists
# which will happen if the kava docker container is stopped and later restarted
if test -f "/root/.kava/config/genesis.json"; then
    echo "genesis.json alredy exists, skipping chain init and validator initilization"
else
    # create default genesis and node config
    kava init test --chain-id=localnet_7777-1

    # ensure evm api listens on all addresses
    sed -i 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/g' /root/.kava/config/app.toml

    # Replace stake with ukava
    sed -in-place='' 's/stake/ukava/g' /root/.kava/config/genesis.json
    # Replace the default evm denom of aphoton with ukava
    sed -in-place='' 's/aphoton/akava/g' /root/.kava/config/genesis.json
    sed -in-place='' 's/"max_gas": "-1"/"max_gas": "20000000"/' /root/.kava/config/genesis.json

    # use the test backend to avoid prompts when storing and accessing keys
    kava config keyring-backend test

    # create an account for the delegator
    kava keys add kava-localnet-delegator

    # add the delegator account to the default genesis
    kava add-genesis-account $(kava keys show kava-localnet-delegator -a) 1000000000ukava

    # create an account for the evm faucet
    echo $evmFaucetMnemonic | kava keys add evm --eth --recover

    # add the evm faucet account to the default genesis
    kava add-genesis-account $(kava keys show evm -a) 1000000000ukava

    # create genesis info for a validator staked by the delegator above
    kava gentx kava-localnet-delegator 500000000ukava \
        --chain-id=localnet_7777-1 \
        --moniker="kava-localnet-validator"

    # merge above transaction with previously generated default genesis
    kava collect-gentxs

    # share node id with peer nodes
    kava tendermint show-node-id >/docker/shared/VALIDATOR_NODE_ID
    # share genesis file with peer nodes
    cp /root/.kava/config/genesis.json /docker/shared/genesis.json
fi

# set config for kava processes to use
cp /docker/kava/config.toml ~/.kava/config/config.toml

# start the kava process
kava start

# run forever (kava start is non-blocking)
tail -f /dev/null
