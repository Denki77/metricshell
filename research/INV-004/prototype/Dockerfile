FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/inv004 ./cmd/inv004

FROM alpine:3.22
COPY --from=build /out/inv004 /usr/local/bin/inv004
ENTRYPOINT ["/usr/local/bin/inv004"]
