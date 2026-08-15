ARG GO_IMAGE=golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556
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
RUN go test -race -count=1 ./...
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
      -ldflags="-s -w -buildid=" -o /out/workload-fixture ./internal/testfixture/workload && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/process-group-fixture ./internal/testfixture/processgroup && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/signal-fixture ./internal/testfixture/signal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/reaper-fixture ./internal/testfixture/reaper

FROM scratch AS integration
COPY --from=integration-build /out/metricshell /metricshell
COPY --from=integration-build /out/workload-fixture /workload-fixture
COPY --from=integration-build /out/process-group-fixture /process-group-fixture
COPY --from=integration-build /out/signal-fixture /signal-fixture
COPY --from=integration-build /out/reaper-fixture /reaper-fixture
ENTRYPOINT ["/metricshell"]

FROM scratch AS runtime
COPY --from=build /out/metricshell /metricshell
ENTRYPOINT ["/metricshell"]
