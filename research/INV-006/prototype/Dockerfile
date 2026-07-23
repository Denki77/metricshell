FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/inv006-bench ./cmd/inv006-bench

FROM alpine:3.22
COPY --from=build /out/inv006-bench /usr/local/bin/inv006-bench
ENTRYPOINT ["/usr/local/bin/inv006-bench"]
