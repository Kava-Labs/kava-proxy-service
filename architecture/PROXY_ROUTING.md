# Proxy Routing

The proxy chooses where to route a request primarily by the incoming Host URL to which the client
originally made their request. The routing is configured by maps of the Host to a backend url in
the environment variables.

All possible configurations use the `PROXY_BACKEND_HOST_URL_MAP` environment variable. This encodes
the default backend to route all requests from a given host. Additional functionality is available
via the `PROXY_HEIGHT_BASED_ROUTING_ENABLED` env variable (see [Rudimentary Sharding](#rudimentary-sharding)).

Consider the simplest case: a host-based-only routing proxy configured for one host
```
PROXY_HEIGHT_BASED_ROUTING_ENABLED=false
PROXY_BACKEND_HOST_URL_MAP=localhost:7777>http://kava:8545
```
This value is parsed into a map that looks like the following:
```
{
    "localhost:7777" => "http://kava:8545",
}
```
Any request to the service will be routed according to this map.
ie. all requests to local port 7777 get forwarded to `http://kava:8545`

Implementations of the [`Proxies` interface](../service/proxy.go#L13) contain logic for deciding
the backend host url to which a request is routed. This is used in the ProxyRequestMiddleware to
route requests.

Any request made to a host not in the map responds 502 Bad Gateway.

## More Examples of Host-only routing

Here is a diagram of the above network topology:

![Proxy Service configured for one host](images/proxy_service_simple_one_host.jpg)

In this simple configuration of only having default hosts, you can do many things:

**Many hosts -> One backend**

```
PROXY_HEIGHT_BASED_ROUTING_ENABLED=false
PROXY_BACKEND_HOST_URL_MAP=localhost:7777>http://kava:8545,localhost:7778>http://kava:8545
```
This value is parsed into a map that looks like the following:
```
{
    "localhost:7777" => "http://kava:8545",
    "localhost:7778" => "http://kava:8545",
}
```
All requests to local ports 7777 & 7778 route to the same cluster at kava:8545

![Proxy Service configured for many hosts for one backend](images/proxy_service_many_hosts_one_backend.jpg)

**Many hosts -> Many backends**

```
PROXY_HEIGHT_BASED_ROUTING_ENABLED=false
PROXY_BACKEND_HOST_URL_MAP=evm.kava.io>http://kava-pruning:8545,evm.data.kava.io>http://kava-archive:8545
```
This value is parsed into a map that looks like the following:
```
{
    "evm.kava.io"      => "http://kava-pruning:8545",
    "evm.data.kava.io" => "http://kava-archive:8545",
}
```
Requests made to evm.kava.io route to a pruning cluster.
Those made to evm.data.kava.io route to an archive cluster.

![Proxy Service configured for many hosts with many backends](images/proxy_service_many_hosts_many_backends.jpg)

## Rudimentary Sharding

Now suppose you want multiple backends for the same host.

The proxy service supports height-based routing to direct requests that only require the most recent
block to a different cluster.

This is configured via the `PROXY_HEIGHT_BASED_ROUTING_ENABLED` and `PROXY_PRUNING_BACKEND_HOST_URL_MAP`
environment variables.
* `PROXY_HEIGHT_BASED_ROUTING_ENABLED` - flag to toggle this functionality
* `PROXY_PRUNING_BACKEND_HOST_URL_MAP` - like `PROXY_BACKEND_HOST_URL_MAP`, but only used for JSON-RPC
  requests that target the latest block (or are stateless, like `eth_chainId`, `eth_coinbase`, etc).

For example, to lighten the load for your resource-intensive (& expensive) archive cluster, you can
route all requests for the "latest" block to a less resource-intensive (& cheaper) pruning cluster:
```
PROXY_HEIGHT_BASED_ROUTING_ENABLED=true
PROXY_BACKEND_HOST_URL_MAP=evm.data.kava.io>http://kava-archive:8545
PROXY_PRUNING_BACKEND_HOST_URL_MAP=evm.data.kava.io>http://kava-pruning:8545
```
This value is parsed into a map that looks like the following:
```
{
  "default": {
    "evm.data.kava.io" => "http://kava-archive:8545",
  },
  "pruning": {
    "evm.data.kava.io" => "http://kava-pruning:8545",
  }
}
```
All traffic to evm.data.kava.io that targets the latest block (or requires no history) routes to the pruning cluster.
Otherwise, all traffic is sent to the archive cluster.

![Proxy Service configured with rudimentary sharding](images/proxy_service_rudimentary_sharding.jpg)

### Default vs Pruning Backend Routing

When `PROXY_HEIGHT_BASED_ROUTING_ENABLED` is `true`, the following cases will cause requests to route
to the the backend url defined in `PROXY_PRUNING_BACKEND_HOST_URL_MAP` (if present):
* requests that include any of the following block tags:
  * `"latest"`
  * `"finalized"`
  * `"pending"`
  * `"safe"`
  * empty/missing block tag (interpreted as `"latest"`)
* requests for methods that require no historic state, including transaction broadcasting
  * for a full list of methods, see [`NoHistoryMethods`](../decode/evm_rpc.go#L89)

All other requests fallback to the default backend url defined in `PROXY_BACKEND_HOST_URL_MAP`.
This includes
* requests for hosts not included in `PROXY_PRUNING_BACKEND_HOST_URL_MAP`
* requests targeting any specific height by number
  * NOTE: the service does not track the current height of the chain. if the tip of the chain is at
    block 1000, a query for block 1000 will still route to the default (not pruning) backend
* requests for methods that use block hash, like `eth_getBlockByHash`
* requests with unparsable (invalid) block numbers
* requests for block tag `"earliest"`

The service will panic on startup if a host in `PROXY_PRUNING_BACKEND_HOST_URL_MAP` is not present
in `PROXY_BACKEND_HOST_URL_MAP`.

Any request made to a host not in the `PROXY_BACKEND_HOST_URL_MAP` map responds 502 Bad Gateway.

## Metrics

When metrics are enabled, the `proxied_request_metrics` table tracks the backend to which requests
are routed in the `response_backend` column.

When height-based sharding is disabled (`PROXY_HEIGHT_BASED_ROUTING_ENABLED=false`), the value is
always `DEFAULT`.

When enabled, the column will have one of the following values:
* `DEFAULT` - the request was routed to the backend defined in `PROXY_BACKEND_HOST_URL_MAP`
* `PRUNING` - the request was routed to the backend defined in `PROXY_PRUNING_BACKEND_HOST_URL_MAP`
