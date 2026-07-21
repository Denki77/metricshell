FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -o /out/metricshell ./cmd/metricshell \
    && CGO_ENABLED=0 go build -o /out/workload ./cmd/workload

FROM alpine:3.22
COPY --from=build /out/metricshell /usr/local/bin/metricshell
COPY --from=build /out/workload /usr/local/bin/workload
EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/metricshell"]
