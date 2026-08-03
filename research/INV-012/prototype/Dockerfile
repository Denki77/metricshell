FROM golang:1.23-alpine AS build
WORKDIR /src
COPY ["go.mod", "./"]
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/inv012 ./cmd/inv012

FROM alpine:3.20
RUN addgroup -S app && adduser -S -G app app
COPY --from=build /out/inv012 /usr/local/bin/inv012
USER app
ENTRYPOINT ["/usr/local/bin/inv012"]
