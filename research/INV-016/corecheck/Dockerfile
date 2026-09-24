FROM golang:1.25 AS build
WORKDIR /src/implementation
COPY ["implementation/internal/snapshot", "./internal/snapshot/"]
COPY ["research/INV-016/corecheck/docker.go.mod", "./go.mod"]
COPY ["research/INV-016/corecheck/main.go", "./cmd/corecheck/main.go"]
RUN CGO_ENABLED=0 go build -o /out/corecheck ./cmd/corecheck

FROM alpine:3.22
COPY --from=build /out/corecheck /usr/local/bin/corecheck
RUN mkdir /results
ENTRYPOINT ["/usr/local/bin/corecheck"]
