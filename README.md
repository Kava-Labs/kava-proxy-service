# kava-proxy-service

L7 proxy that enables observability, routing, and scaling of kava APIs to the farthest reaches of the cosmos.

## Usage

_Under Development_.

### Environment Variables

Many aspects of the service are configurable via environment variables:

- `LOG_LEVEL` - controls the amount of logs produced by the service, defaults to `INFO`, supported values in order from most verbose to least verbose are
    - DEBUG
    - INFO
    - ERROR

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
