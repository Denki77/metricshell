FROM golang:1.23-alpine AS build
WORKDIR /src
COPY ["go.mod", "go.sum", "./"]
RUN go mod download
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/inv010 ./cmd/inv010

FROM alpine:3.20
COPY --from=build /out/inv010 /usr/local/bin/inv010
ENTRYPOINT ["/usr/local/bin/inv010"]
