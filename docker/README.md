# e2e test setup

For the most accurate reflection of real-world use cases, this repo includes an end-to-end test
setup that includes a kava node running with multiple validators.

This directory contains the configuration and startup files necessary for that setup.

From the repo root, the following make commands work with these files:
* `make up` - starts the service. if it has never been started before, setup will be performed
* `make down` - stops & destroys the nodes & network
* `make reset` - destroys & recreates the nodes & network. setup will be performed
* `make ready` - blocks the process until the network is producing blocks

`make e2e-test` and `make test` (which runs unit & e2e tests) both rely on the network being running and ready.

The following does not affect the network:
* `make refresh` - destroys and recreates only the proxy service. useful for picking up new proxy env variables.


## how it works

The setup runs a network of nodes via docker-compose where each node has its own container:
* `kava-validator` - the validator node (see below)
* `kava-pruning` - an API-enabled peer node

There is a network running with a single validator. The config and entrypoint for this node is in [kava-validator](./kava-validator/).

The `shared/` directory is shared between all nodes and is used to share necessary details from the validator to the other nodes in the network.

The validator has an [entrypoint](./kava-validator/kava-validator-entrypoint.sh) that does the following on first startup:
* `init`s a new kava home directory
* creates a gentx and initializes the network genesis file
* writes its node id to `shared/VALIDATOR_NODE_ID` so peers can connect to it
* copies the genesis file to `shared/genesis` so other peers can use it to connect
* starts the network

Meanwhile, any peer node in the network (configured in [docker-compose.yml](../docker-compose.yml)) has an [entrypoint](./shared/kava-entrypoint.sh)
that does the following:
* `init`s a new kava home directory
* waits for the validator to share the genesis file
* copies over the validator
* reads the validator's node id from `shared/VALIDATOR_NODE_ID`
* starts the network with the validator as a peer

## add more nodes

We'll want more shards in the future!! This setup supports this. To add another api-enabled node to the network:
1. Add a configuration in the [docker-compose.yml](../docker-compose.yml)
```yml
  # peer node with api running validator's network
  nodename:
    image: kava/kava:${KAVA_CONTAINER_TAG}
    entrypoint: /docker/shared/kava-entrypoint.sh
    env_file: .env
    volumes:
      - ./docker/shared:/docker/shared
    # expose ports for other services to be able to connect to within
    # the default docker-compose network
    expose:
      - "${KAVA_CONTAINER_COSMOS_RPC_PORT}"
      - "${KAVA_CONTAINER_EVM_RPC_PORT}"
    # optional: bind host ports to access outside docker network
    ports:
      - "${EXPOSED_RPC_PORT}:${KAVA_CONTAINER_COSMOS_RPC_PORT}"
      - "${EXPOSED_EVM_JSON_RPC_PORT}:${KAVA_CONTAINER_EVM_RPC_PORT}"
```
Note that `nodename` should be replaced with whatever you call your new node.

The `ports` bindings are only necessary if you want to directly query the node from outside the docker network.
If so, replace and create new env variables for
* `EXPOSED_RPC_PORT` - the host (outside docker network) port for the rpc api
* `EXPOSED_EVM_JSON_RPC_PORT` - the host (outside docker network) port for the evm json rpc api

2. Add the new node to the proxy backend host map config: `localhost:${PROXY_PORT_FOR_NEW_HOST}>http://nodename:8545`

3. Make sure the proxy port routes to the proxy service. Configure this in the docker-compose `ports`
   of `proxy`: `- "${PROXY_PORT_FOR_NEW_HOST}:${PROXY_CONTAINER_PORT}"`

4. Run `make reset` and the new node should be running & connected to the network.
