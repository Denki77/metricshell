FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY ["api", "./api/"]
COPY ["cmd", "./cmd/"]
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/inv008-bench ./cmd/inv008-bench

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/inv008-bench /usr/local/bin/inv008-bench
ENTRYPOINT ["/usr/local/bin/inv008-bench"]
