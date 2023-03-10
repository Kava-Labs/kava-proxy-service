# Development Guide

This document provides context, tips and processes that developers can use when writing or debugging code for this service and it's dependencies.

## Dependencies

The following dependencies are required to build and run the proxy service:

- [Docker](https://docs.docker.com/engine/install/) for building
- [Docker Compose (v2+)](https://docs.docker.com/compose/install/)
- [Delve](https://github.com/go-delve/delve/tree/master/Documentation/installation) for step debugging of running golang processes

## Configuration

Adjusting or setting of values to be used by `make`, `docker-compose` or any of the containerized applications is possible by modifying the [local environment file](./.env)

```bash
##### Local development config

# Values used by `make`
CONTAINER_NAME=kava-proxy-service
IMAGE_NAME=kava-labs/kava-proxy-service
LOCAL_IMAGE_TAG=local
PRODUCTION_IMAGE_TAG=latest

# Values used by docker-compose
POSTGRES_CONTAINER_PORT=5432
POSTGRES_HOST_PORT=5432

##### Kava Proxy Config
LOG_LEVEL=DEBUG

etc...
```

## Building

To format and verify the syntax of the golang source code and dependency tree run:

```bash
make lint
```

While normally the docker image for local development will be auto-built by `docker-compose` when starting (`make up` or refreshing the service (`make refresh`) the service, the below commands can be used to build the docker image on demand or compile the service binary directly on the current host.

```bash
# build a development version docker image of the service
make build
# build a binary of the service using local sources
# that can run on the build host and place it in the
# GOBIN path for the current host
make install
# build a production version docker image of the service
make publish
```

Alternatively one can compile tbe service binary in the current directory:

```bash
# requires go 1.20 or greater
⋊> go build
⋊> ./kava-proxy-service        13:55:22
{"level":"info","time":"2023-03-02T13:55:25-08:00","caller":"/Users/levischoen/forges/kava/kava-proxy-service/main.go:38","message":"There and back again"}
```

## Testing

```bash
# To run all tests (unit and environment based ones)
make test
```

## Unit Tests

Prefix unit tests with `UnitTest`, e.g. `TestUnitTestValidateConfigThrowsErrorOnInvalidConfig`

```bash
make unit-test
```

### End-to-End Tests

End-to-End (E2E) Tests run against a live version of the proxy service API (based on environment variables), and are useful both for local development and running as acceptance and canary tests on production deployments of the proxy service

Prefix E2E tests with `TestE2ETest`, e.g. `TestE2ETestHealthCheckReturns200`

```bash
make e2e-test
```

The e2e tests won't pass if the proxy service and it's dependencies aren't fully running- e.g. the proxy service can start up in > second but the kava service can take 10's of seconds. To prevent test failures due to that situation, if you are restarting or starting the services for the first time and want to execute the tests immediately call the make `ready` target before the `e2e-test` target.

```bash
make ready e2e-test
```

## Migrations

On startup the proxy service will run any SQL based migration in the [migrations folder](./clients/database/migrations) that haven't already been run against the database being used.

For lower level details on how the migration process works consult [these docs](https://bun.uptrace.dev/guide/migrations.html).

### Adding a new migration

Generate unique monotonically increasing migration prefix (to ensure) new migrations are detected and ran

```bash
$ date '+%Y%m%d%H%M%S'
> 20230306182227
```

Add new SQL file with commands to run in the new migration (add/delete/modify tables and or indices) in the in the [migrations folder](./clients/database/migrations)

### Running migrations

The below environment variable is used to control whether the proxy service will attempt to run migrations when it starts:

```bash
RUN_DATABASE_MIGRATIONS=true
```

In production by default database migrations are not run when the service starts to allow finer grained control when running migrations. For local development migrations ARE automatically run at service start time.

## Running

An example of command flow used during typical iterative development:

```bash
# start (or restart previously built) containers for all the services
# in docker-compose.yml
make up
# rebuild, reset state and restart all containers for all the services
# in docker-compose.yml
make reset
# rebuild, reset state and restart just the proxy service
make refresh
# stop and start (without re-building or wiping state) all services
# in docker-compose.yml
make restart
# stop all services in docker-compose.yml
make down
```

## Debugging

```bash
# follow the logs from all the dockerized services
make logs
# or just one (filtering based off the name of the service in the docker-compose.yml file)
make logs S=proxy
```

If the proxy service is up, running

```bash
make proxy-debug
```

will launch a delve debugging session attached to the currently running proxy service process. Additional information on delve usage can be found [here](https://github.com/go-delve/delve/blob/master/Documentation/usage/dlv_attach.md)

View all functions

```bash
funcs
```

Set a breakpoint

```bash
b github.com/kava-labs/kava-proxy-service/config.ReadConfig
```

Run program until breakpoint is hit

```bash
continue
```

step through execution of the current line of code

```bash
next
```

step through execution one line of golang code at a time

```bash
step
```

print out more characters from strings

```bash
config max-string-len 1000
p longStringVariable
```

to stop the current process

```bash
^c
```

Additional debug commands are available for connecting to the database or cache used by the proxy service

```bash
# open a connection to the postgres database for debugging it's state
# https://www.postgresql.org/docs/current/app-psql.html
make debug-database
# open a connection to the redis service for debugging it's state
# https://redis.io/docs/ui/cli/
make debug-cache
```

## Feedback

For suggesting changes or reporting issues, please open a Github Issue.
