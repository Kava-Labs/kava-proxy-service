# Development Guide

## Dependencies

- docker
- docker compose v2
- delve golang debugger installed

## Configuration

## Building

## Testing

```Makefile
# run all unit tests
unit-test:
	go test -v ./... -run "^TestUnitTest*"

# run tests that execute against a local or remote instance of the API
e2e-test:
	go test -v ./... -run "^TestE2ETest*"
```

## Running

## Debugging

```Makefile
# follow the logs from all the dockerized servcies
logs:
	docker-compose logs ${S} -f

# build and push docker image from local sources to remote registry
publish:

# open a shell to the postgres database for debugging it's state
debug-database:
	docker-compose exec postgres psql -U postgres

# open a shell to the redis service for debugging it's state
debug-cache:
	docker-compose exec redis redis-cli
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
