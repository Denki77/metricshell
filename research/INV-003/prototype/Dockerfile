FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/metricshell ./cmd/metricshell && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/workload ./cmd/workload

FROM alpine:3.22
RUN apk add --no-cache curl
COPY --from=build /out/metricshell /usr/local/bin/metricshell
COPY --from=build /out/workload /usr/local/bin/workload
ENTRYPOINT ["/usr/local/bin/metricshell"]
