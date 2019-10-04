ARG CONFIG=config.yml
ARG GO_VERSION=1.13
ARG ALPINE_VERSION=3.10
## GOLANG BUILD STAGE##
FROM golang:$GO_VERSION as build-env

RUN apt-get update && apt-get install -y  git gcc openssh-client musl-dev

WORKDIR /src

COPY ["go.mod", "go.sum", "./"]

RUN go mod download

COPY cmd  ./cmd
COPY internal  ./internal
COPY tool  ./tool

RUN : \
    && mkdir -p /dist/plugins \
    && go build -a -o /dist/whois-proxy -v -gcflags "all=-N -l"  ./cmd/whois-proxy \
    && ls -la /dist/

# Get and Compile Delve
RUN go get -u github.com/go-delve/delve/cmd/dlv


## APP STAGE ##
FROM alpine:$ALPINE_VERSION as app
ARG CONFIG
RUN : \
    && apk update \
    && apk add mc \
    && mkdir -p /opt/whois-proxy/  ;

COPY .docker/${CONFIG} /opt/whois-proxy/config.yml

COPY --from=build-env /dist/whois-proxy  /opt/whois-proxy/whois-proxy

WORKDIR /opt/whois-proxy
ENTRYPOINT ["./whois-proxy"]


## DLV DEBUG STAGE ##
FROM app as app-debug

# Allow delve to run on Alpine based containers.
RUN apk add --no-cache libc6-compat

# Copy dlv binary file from build-env stage
COPY --from=build-env /go/bin/dlv /opt/whois-proxy/dlv

# IF Need dlv to debug use this
ENTRYPOINT ["sleep", "10000000"]
