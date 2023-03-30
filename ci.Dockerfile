# Specify base image for building service binary
FROM golang:1.20

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
COPY decode/ decode/
COPY routines/ routines/

# build service from latest sources
RUN go install

# by default when a container is started from this image
# map port 7777 from the host to the container and run the
# proxy service
EXPOSE 7777
CMD ["kava-proxy-service"]
