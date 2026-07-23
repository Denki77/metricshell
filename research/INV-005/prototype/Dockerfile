FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY ["cmd", "./cmd/"]
RUN go mod tidy \
    && CGO_ENABLED=0 go build -o /out/bench ./cmd/bench

FROM alpine:3.22
RUN apk add --no-cache php84-cli php84-curl php84-sockets
COPY --from=build /out/bench /usr/local/bin/inv005-bench
COPY ["php", "/opt/inv005/php/"]
ENTRYPOINT ["/usr/local/bin/inv005-bench"]
