# kava-proxy-service

L7 proxy that enables observability, routing, and scaling of kava APIs to the farthest reaches of the cosmos.

## Usage

[Service Workflows](https://github.com/Kava-Labs/kava-proxy-service/blob/main/architecture/ARCHITECTURE.md#service-workflows)

### Environment Variables

Many aspects of the service are configurable via environment variables, for example:

- `LOG_LEVEL` - controls the amount of logs produced by the service, defaults to `INFO`, supported values in order from most verbose to least verbose are:

  - TRACE
  - DEBUG
  - INFO
  - ERROR

-  `PROXY_BACKEND_HOST_URL_MAP` - comma delimited list of `HOSTNAME_TO_PROXY>BACKEND_PROXY_SERVER_FOR_HOSTNAME` controls what backend server the proxy service will proxy a request to based on the hostname of the request. `,` is used as a separator between entries in the map, `>` is used as a separator within an entry to delimit between the hostname to proxy for and the backend to proxy to. At least one entry must be present. The same backend can be used as the proxy for multiple hostnames by creating one entry for each of the hostnames to proxy for. Example value:

> PROXY_BACKEND_HOST_URL_MAP=evm.app.internal.testnet.us-east.production.kava.io>https://evmrpc.internal.testnet.proxy.kava.io,evm.data.internal.testnet.us-east.production.kava.io>https://evmrpcdata.internal.testnet.proxy.kava.io

For a full list of supported environment variables refer to the [code](./config/config.go) and [development environment file](./env)

### Logging

The service logs to stdout using the json format, with logging configurable (via environment variables) at the following levels:

- DEBUG
- INFO
- ERROR

```text
kava-proxy-service-proxy-1  | {"level":"debug","time":"2023-03-02T19:30:18Z","caller":"/app/main.go:34","message":"initial config: {LogLevel:DEBUG}"}
kava-proxy-service-proxy-1  | {"level":"info","time":"2023-03-02T19:30:18Z","caller":"/app/main.go:37","message":"There and back again"}
```

## Development

See [Development.md](./DEVELOPMENT.md).

## Architecture

See [architecture docs](./architecture/).

## Feedback

For suggesting changes or reporting issues, please open a Github Issue.
