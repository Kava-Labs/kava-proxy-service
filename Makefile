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
	docker build ./ -f local.Dockerfile -t ${IMAGE_NAME}:${LOCAL_IMAGE_TAG}

.PHONY: publish
# build a production version docker image of the service
publish: lint
	docker build ./ -f production.Dockerfile -t ${IMAGE_NAME}:${PRODUCTION_IMAGE_TAG}

.PHONY: unit-test
# run all unit tests
unit-test:
	go test -count=1 -v -cover -coverprofile cover.out --race ./... -run "^TestUnitTest*"

.PHONY: e2e-test
# run tests that execute against a local or remote instance of the API
e2e-test:
	go test -count=1 -v -cover -coverprofile cover.out --race ./... -run "^TestE2ETest*"

.PHONY: it
# run any test matching the provided pattern, can pass a regex or a string
# of the exact test to run
it : lint
	go test -count=1 -v -cover -coverprofile cover.out --race ./... -run=".*${p}.*"

.PHONY: show-coverage
# convert test coverage report to html & open in browser
show-coverage:
	go tool cover -html cover.out -o cover.html && open cover.html

.PHONY: test
# run all tests
test:
	go test -count=1 -v -cover -coverprofile cover.out --race ./...

.PHONY: up
# start dockerized versions of the service and it's dependencies
up:
	mkdir -p docker/shared/gentx
	docker compose up -d

.PHONY: down
# stop the service and it's dependencies
down:
	rm -fr docker/shared/gentx
	docker compose down

.PHONY: restart
# restart just the service (useful for picking up new environment variable values)
restart:
	docker compose up -d proxy --force-recreate

.PHONY: reset
# wipe state and restart the service and all it's dependencies
reset: lint
	rm -fr docker/shared/gentx
	mkdir -p docker/shared/gentx
	docker compose up -d --build --remove-orphans --renew-anon-volumes --force-recreate

.PHONY: refresh

# rebuild from latest local sources and restart just the service containers
# (preserving any volume state such as database tables & rows)
refresh:
	docker compose up -d proxy --build --force-recreate

# poll kava service status endpoint until it doesn't error
.PHONY: ready
ready:
	./scripts/wait-for-kava-node-running.sh && \
	./scripts/wait-for-proxy-service-running.sh && \
	./scripts/wait-for-proxy-service-database-metric-partitions.sh

.PHONY: logs
# follow the logs from all the dockerized services
# make logs
# or one
# make logs S=proxy
logs:
	docker compose logs ${S} -f

.PHONY: debug-proxy
# attach the dlv debugger to the running service and connect to the dlv debugger
debug-proxy:
	docker compose exec -d proxy dlv attach 1 --listen=:${PROXY_CONTAINER_DEBUG_PORT} --headless --api-version=2 --log && \
	dlv connect :${PROXY_HOST_DEBUG_PORT}

.PHONY: debug-database
# open a connection to the postgres database for debugging it's state
# https://www.postgresql.org/docs/current/app-psql.html
debug-database:
	docker compose exec postgres psql -U ${DATABASE_USERNAME} -d ${DATABASE_NAME}

.PHONY: debug-cache
# open a connection to the redis service for debugging it's state
# https://redis.io/docs/ui/cli/
debug-cache:
	docker compose exec redis redis-cli
