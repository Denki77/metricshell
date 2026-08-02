FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/inv009-bench ./cmd/inv009-bench

FROM alpine:3.22
COPY --from=build /out/inv009-bench /usr/local/bin/inv009-bench
ENTRYPOINT ["/usr/local/bin/inv009-bench"]
