FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/inv007-bench ./cmd/inv007-bench

FROM alpine:3.22
COPY --from=build /out/inv007-bench /usr/local/bin/inv007-bench
ENTRYPOINT ["/usr/local/bin/inv007-bench"]
