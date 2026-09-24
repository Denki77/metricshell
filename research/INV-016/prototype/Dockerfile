FROM golang:1.25 AS build
WORKDIR /src
COPY ["go.mod", "./"]
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -o /out/inv016 ./cmd/inv016

FROM alpine:3.22
COPY --from=build /out/inv016 /usr/local/bin/inv016
ENTRYPOINT ["/usr/local/bin/inv016"]
