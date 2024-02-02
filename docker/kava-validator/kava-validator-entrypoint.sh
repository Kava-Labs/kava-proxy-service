#!/bin/bash

# log all commands to stdout and stop the script on the first error
set -ex

evmFaucetMnemonic='sweet ocean blush coil mobile ten floor sample nuclear power legend where place swamp young marble grit observe enforce lake blossom lesson upon plug'
# Private Key in hex: 247069f0bc3a5914cb2fd41e4133bbdaa6dbed9f47a01b9f110b5602c6e4cdd9
# EVM Address in hex: 0x6767114FFAA17c6439D7aEA480738b982ce63A02

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
cp /docker/kava/app.toml ~/.kava/config/app.toml

# start the kava process
kava start

# run forever (kava start is non-blocking)
tail -f /dev/null
