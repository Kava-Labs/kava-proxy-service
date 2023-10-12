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

Each node is given it's own directory that contains the node's config.toml.
The `shared/` directory is shared between all nodes and is used to share necessary details across validators.

All nodes have entrypoint [`shared/kava-entrypoint.sh`](./shared/kava-entrypoint.sh)
On first startup, this file
* `init`s a new kava home directory
* creates a gentx that gets copied to `shared/gentx` (created by `Makefile` commands)
  * the gentxs are copied with a special name that tells the other nodes this validators address, node id, & hostname
* waits for all validators to have copied over their gentxs
* generates a genesis file & creates a genesis account for each validator
* starts the network with all other validators as persistent peers


### other shared files
* `GENESIS_TIME` - in order to not have an AppHash mismatch, the genesis files need to have the same
  genesis time.
* `NUM_VALIDATORS` - contains only the number of validators. this is used to ensure the validator waits
  for all other gentxs before building genesis file.


## to add more validators

We'll want more shards in the future!! This setup supports this. To add another validator to the network:
1. Create a new config.toml dir for the node

2. Add a configuration in the [docker-compose.yml](../docker-compose.yml)
```yml
  nodename:
    image: kava/kava:${KAVA_CONTAINER_TAG}
    entrypoint: /docker/shared/kava-entrypoint.sh
    env_file: .env
    environment:
      # used by the container to notify other containers of its peer address
      - CONTAINER_NAME=nodename
    volumes:
      - ./docker/kavapruning:/docker/kava
      - ./docker/shared:/docker/shared
    ports:
      - "${EXPOSED_RPC_PORT}:${KAVA_CONTAINER_COSMOS_RPC_PORT}"
      - "${EXPOSED_EVM_JSON_RPC_PORT}:${KAVA_CONTAINER_EVM_RPC_PORT}"
    # expose ports for other services to be able to connect to within
    # the default docker-compose network
    expose:
      - "${KAVA_CONTAINER_COSMOS_RPC_PORT}"
      - "${KAVA_CONTAINER_EVM_RPC_PORT}"
```
Note that `nodename` should be replaced with whatever you call your new node. Replace it both in the
top-level service name & in the `CONTAINER_NAME` env variable.
Additionally, replace and create new env variables for
* `EXPOSED_RPC_PORT` - the host (outside docker network) port for the rpc api
* `EXPOSED_EVM_JSON_RPC_PORT` - the host (outside docker network) port for the evm json rpc api

3. Increment the number in [`NUM_VALIDATORS`](./shared/NUM_VALIDATORS)

4. Run `make reset` and the new node should get picked up in the gentxs & peers of all the other validators.
