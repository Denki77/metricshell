FROM golang:1.23-alpine AS build
WORKDIR /src
COPY ["go.mod", "./"]
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/inv011 ./cmd/inv011

FROM alpine:3.20
COPY --from=build /out/inv011 /usr/local/bin/inv011
ENTRYPOINT ["/usr/local/bin/inv011"]
