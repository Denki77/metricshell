FROM golang:1.24-alpine AS build
WORKDIR /src
COPY ["go.mod", "./"]
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/inv018 ./cmd/inv018

FROM alpine:3.22
RUN addgroup -S metricshell && adduser -S -G metricshell metricshell && mkdir -p /run/metricshell && chown metricshell:metricshell /run/metricshell
COPY --from=build /out/inv018 /usr/local/bin/inv018
USER metricshell
ENTRYPOINT ["/usr/local/bin/inv018"]
