# Specify base image for building service binary
FROM golang:1.20

# Install go debugger for easier debug life
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Install kava cli for debugging
ARG PROXY_KAVA_CLI_VERSION_ARG=v0.21.0
ENV PROXY_KAVA_CLI_VERSION=$PROXY_KAVA_CLI_VERSION_ARG

RUN git clone https://github.com/Kava-Labs/kava.git
RUN cd kava && git checkout $PROXY_KAVA_CLI_VERSION && make install

# create and set default directory for service  files
RUN mkdir /app
WORKDIR /app

# optimize build time by installing dependencies
# before building so if source code changed but not
# the list of dependencies they don't have to be re-downloaded
COPY go.mod go.sum ./

# download service golang dependnecies source code
RUN go mod download

# copy over local sources used to build service
COPY *.go ./
COPY logging/ logging/
COPY clients/ clients/
COPY config/ config/
COPY service/ service/

# build service from latest sources
# with all compilier optimizations off to support debugging
RUN go install  -gcflags=all="-N -l"

# by default when a container is started from this image
# map port 7777 from the host to the container and run the
# proxy service
EXPOSE 7777
CMD ["kava-proxy-service"]
