FROM golang:1.23-alpine AS build
WORKDIR /src
COPY ["go.mod", "./"]
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/inv014 ./cmd/inv014

FROM alpine:3.20
RUN addgroup -S app && adduser -S -G app app && install -d -m 0700 -o app -g app /run/metricshell
COPY --from=build /out/inv014 /usr/local/bin/inv014
USER app
ENTRYPOINT ["/usr/local/bin/inv014"]
