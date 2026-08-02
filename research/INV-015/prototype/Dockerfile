FROM golang:1.23-alpine AS build
WORKDIR /src
COPY ["go.mod", "./"]
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/inv015 ./cmd/inv015

FROM alpine:3.20
COPY --from=build /out/inv015 /usr/local/bin/inv015
ENTRYPOINT ["/usr/local/bin/inv015"]
