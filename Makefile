# execute these tasks when `make` with no target is invoked
default: unit-test reset e2e-test

# declare non-file based targets to speed up target
# invocataions by letting make know it can skip checking
# for changes in a file
.PHONY: lint install build publish unit-test e2e-test test up down restart reset refresh logs debug-postgres debug-cache

# import environment file for setting or overriding
# configuration used by this Makefile
include .env

# source all variables in environment file
# This only runs in the make command shell
# so won't affect your login shell
export $(shell sed 's/=.*//' .env)

# format and verify service source code and dependency tree
lint:
	go mod tidy
	go fmt ./...
	go vet ./...

# build a binary of the service using local sources
# that can run on the build host and place it in the
# GOBIN path for the current host
install:
	go install

# build a development version docker image of the service
build:
	docker build ./ -f LocalDockerfile -t ${IMAGE_NAME}:${LOCAL_IMAGE_TAG}

# build a production version docker image of the service
publish:
	docker build ./ -f ProductionDockerfile -t ${IMAGE_NAME}:${PRODUCTION_IMAGE_TAG}

# run all unit tests
unit-test:
	go test -v ./... -run "^TestUnitTest*"

# run tests that execute against a local or remote instance of the API
e2e-test:
	go test -v ./... -run "^TestE2ETest*"

# run all tests
test: unit-test e2e-test

# start dockerized versions of the service and it's dependencies
up:
	docker-compose up -d

# stop the service and it's dependencies
down:
	docker-compose down

# restart the service and all it's dependencies (preserving any container state)
restart: down up

# wipe state and restart the service and all it's dependencies
reset:
	docker-compose up -d --build --remove-orphans --renew-anon-volumes --force-recreate

# rebuild and restart just the service
refresh:
	docker-compose up proxy -d --build --remove-orphans --force-recreate

# follow the logs from all the dockerized services
# make logs
# or one
# make logs S=proxy
logs:
	docker-compose logs ${S} -f

# attach the dlv debugger to the running service and connect to the dlv debugger
debug-proxy:
	docker-compose exec -d proxy dlv attach 1 --listen=:2345 --headless --api-version=2 --log && \
	dlv connect :2345

# open a connection to the postgres database for debugging it's state
debug-database:
	docker-compose exec postgres psql -U postgres

# open a connection to the redis service for debugging it's state
debug-cache:
	docker-compose exec redis redis-cli
