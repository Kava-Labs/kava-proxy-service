# execute these tasks when `make` with no target is invoked
default: unit-test reset ready e2e-test

# import environment file for setting or overriding
# configuration used by this Makefile
include .env

# source all variables in environment file
# This only runs in the make command shell
# so won't affect your login shell
export $(shell sed 's/=.*//' .env)

.PHONY: lint
# format and verify service source code and dependency tree
lint:
	go mod tidy
	go fmt ./...
	go vet ./...

.PHONY: install
# build a binary of the service using local sources
# that can run on the build host and place it in the
# GOBIN path for the current host
install: lint
	go install

.PHONY: build
# build a development version docker image of the service
build: lint
	docker build ./ -f local.Dockerfile -t ${IMAGE_NAME}:${LOCAL_IMAGE_TAG} --build-arg PROXY_KAVA_CLI_VERSION_ARG=${PROXY_KAVA_CLI_VERSION}

.PHONY: publish
# build a production version docker image of the service
publish: lint
	docker build ./ -f production.Dockerfile -t ${IMAGE_NAME}:${PRODUCTION_IMAGE_TAG}

.PHONY: unit-test
# run all unit tests
unit-test:
	go test -count=1 -v -cover --race ./... -run "^TestUnitTest*"

.PHONY: e2e-test
# run tests that execute against a local or remote instance of the API
e2e-test:
	go test -count=1 -v -cover --race ./... -run "^TestE2ETest*"

.PHONY: test
# run all tests
test: unit-test e2e-test

.PHONY: up
# start dockerized versions of the service and it's dependencies
up:
	docker-compose up -d

.PHONY: down
# stop the service and it's dependencies
down:
	docker-compose down

.PHONY: restart
# restart the service and all it's dependencies (preserving any container state)
restart: down up

.PHONY: reset
# wipe state and restart the service and all it's dependencies
reset:
	docker-compose up -d --build --remove-orphans --renew-anon-volumes --force-recreate

.PHONY: refresh
# rebuild and restart just the service
refresh:
	docker-compose up proxy -d --build --remove-orphans --force-recreate

# poll kava service status endpoint until it doesn't error
.PHONY: ready
ready:
	./scripts/wait-for-kava-node-running.sh

.PHONY: logs
# follow the logs from all the dockerized services
# make logs
# or one
# make logs S=proxy
logs:
	docker-compose logs ${S} -f

.PHONY: debug-proxy
# attach the dlv debugger to the running service and connect to the dlv debugger
debug-proxy:
	docker-compose exec -d proxy dlv attach 1 --listen=:${PROXY_CONTAINER_DEBUG_PORT} --headless --api-version=2 --log && \
	dlv connect :${PROXY_HOST_DEBUG_PORT}

.PHONY: debug-database
# open a connection to the postgres database for debugging it's state
# https://www.postgresql.org/docs/current/app-psql.html
debug-database:
	docker-compose exec postgres psql -U postgres

.PHONY: debug-cache
# open a connection to the redis service for debugging it's state
# https://redis.io/docs/ui/cli/
debug-cache:
	docker-compose exec redis redis-cli
