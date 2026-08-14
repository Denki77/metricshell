ARG GO_IMAGE=golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2
ARG REPO_URI=github.com/Denki77/metricshell

FROM ${GO_IMAGE} AS source
ARG REPO_URI
WORKDIR /src
COPY ["implementation/go.mod", "./"]
COPY ["implementation/cmd", "./cmd"]
COPY ["implementation/internal", "./internal"]
COPY ["VERSION", "/project/VERSION"]

FROM source AS test
ARG REVISION=unknown
RUN test -z "$(gofmt -l .)"
RUN go vet ./...
RUN go test -count=1 ./...
RUN go test -count=1 ./internal/dependencyboundary -run '^TestProductionDependencyBoundary$'
RUN version="$(cat /project/VERSION)" && \
    test -n "${version}" && \
    test "$(wc -l < /project/VERSION)" -eq 1 && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid= -X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" \
      -o /out/linux_amd64/metricshell ./cmd/metricshell && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid= -X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" \
      -o /out/linux_arm64/metricshell ./cmd/metricshell && \
    actual="$(go run -ldflags="-X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" ./cmd/metricshell --version)" && \
    test "${actual}" = "metricshell version=${version} revision=${REVISION}"

FROM source AS build
ARG REVISION=unknown
ARG TARGETOS
ARG TARGETARCH
RUN version="$(cat /project/VERSION)" && \
    test -n "${version}" && \
    test "$(wc -l < /project/VERSION)" -eq 1 && \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid= -X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" \
      -o /out/metricshell ./cmd/metricshell

FROM source AS integration-build
ARG REVISION=unknown
RUN version="$(cat /project/VERSION)" && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid= -X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" \
      -o /out/metricshell ./cmd/metricshell && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/workload-fixture ./internal/testfixture/workload

FROM scratch AS integration
COPY --from=integration-build /out/metricshell /metricshell
COPY --from=integration-build /out/workload-fixture /workload-fixture
ENTRYPOINT ["/metricshell"]

FROM scratch AS runtime
COPY --from=build /out/metricshell /metricshell
ENTRYPOINT ["/metricshell"]
