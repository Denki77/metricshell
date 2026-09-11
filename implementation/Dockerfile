# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
ARG GO_IMAGE=golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556
ARG REPO_URI=github.com/Denki77/metricshell

FROM ${GO_IMAGE} AS source
ARG REPO_URI
WORKDIR /src
COPY ["implementation/Dockerfile", "./Dockerfile"]
COPY ["implementation/Makefile", "./Makefile"]
COPY ["implementation/go.mod", "./"]
COPY ["implementation/client", "./client"]
COPY ["implementation/cmd", "./cmd"]
COPY ["implementation/examples", "./examples"]
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
    sha256sum /out/linux_amd64/metricshell /out/linux_arm64/metricshell > /out/SHA256SUMS && \
    sha256sum -c /out/SHA256SUMS && \
    actual="$(go run -ldflags="-X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" ./cmd/metricshell --version)" && \
    test "${actual}" = "metricshell version=${version} revision=${REVISION}"

FROM source AS release
ARG REVISION=unknown
RUN version="$(cat /project/VERSION)" && \
    mkdir -p /release/linux_amd64 /release/linux_arm64 && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid= -X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" \
      -o /release/linux_amd64/metricshell ./cmd/metricshell && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid= -X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" \
      -o /release/linux_arm64/metricshell ./cmd/metricshell && \
    (cd /release && sha256sum linux_amd64/metricshell linux_arm64/metricshell > SHA256SUMS) && \
    (cd /release && sha256sum -c SHA256SUMS) && \
    printf 'version=%s\nrevision=%s\n' "${version}" "${REVISION}" > /release/METADATA

FROM source AS benchmark
ARG REVISION=unknown
RUN mkdir -p /benchmark && \
    version="$(cat /project/VERSION)" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid= -X=${REPO_URI}/implementation/internal/buildinfo.Version=${version} -X=${REPO_URI}/implementation/internal/buildinfo.Revision=${REVISION}" \
      -o /benchmark/metricshell ./cmd/metricshell && \
    go test -run '^TestBenchmarkSuiteContract$' -bench '^BenchmarkReleaseSuite$' -benchtime=30x -count=1 ./internal/benchrelease > /benchmark/results.txt && \
    printf '{"schema_version":1,"version":"%s","revision":"%s","repetitions":30,"warmups":3,"transports":["file","unix","http"],"profiles":["idle","load"],"platform":"linux/amd64","cpu_set":"0","memory":"64MiB","correctness_separate_from_thresholds":true}\n' "${version}" "${REVISION}" > /benchmark/metadata.json

FROM scratch AS benchmark-artifacts
COPY --from=benchmark /benchmark /benchmark

FROM source AS supply-chain
ARG REVISION=unknown
COPY --from=release /release /supply/release
RUN version="$(cat /project/VERSION)" && \
    cp go.mod /supply/release/go.mod && \
    sha256sum go.mod >> /supply/release/SHA256SUMS && \
    go list -m -json all > /supply/modules.jsonl
RUN status=0; \
    go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 -json ./... > /supply/govulncheck.json || status="$?"; \
    if test "${status}" -ne 0 -a "${status}" -ne 3; then exit "${status}"; fi
RUN --mount=type=secret,id=release_signing_key,target=/run/secrets/release_signing_key,required=true \
    --mount=type=secret,id=release_public_key,target=/run/secrets/release_public_key,required=true \
    version="$(cat /project/VERSION)" && \
    go run ./internal/testfixture/supplychain /supply/release "${version}" "${REVISION}" \
      /run/secrets/release_signing_key /run/secrets/release_public_key /supply/modules.jsonl /supply/govulncheck.json

FROM scratch AS supply-chain-artifacts
COPY --from=supply-chain /supply/release /release

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
      -ldflags="-s -w -buildid=" -o /out/signal-fixture ./internal/testfixture/signal && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/termination-fixture ./internal/testfixture/termination
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/reaper-fixture ./internal/testfixture/reaper && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/result-matrix-fixture ./internal/testfixture/resultmatrix && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/probe-fixture ./internal/testfixture/probe && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/final-scrape-fixture ./internal/testfixture/finalscrape && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
      -ldflags="-s -w -buildid=" -o /out/faultsuite-fixture ./internal/testfixture/faultsuite

FROM scratch AS integration
ENV METRICSHELL_FINAL_WAIT_MODE=immediate
COPY --from=integration-build /out/metricshell /metricshell
COPY --from=integration-build /out/workload-fixture /workload-fixture
COPY --from=integration-build /out/process-group-fixture /process-group-fixture
COPY --from=integration-build /out/signal-fixture /signal-fixture
COPY --from=integration-build /out/reaper-fixture /reaper-fixture
COPY --from=integration-build /out/result-matrix-fixture /result-matrix-fixture
COPY --from=integration-build /out/termination-fixture /termination-fixture
COPY --from=integration-build /out/probe-fixture /probe-fixture
COPY --from=integration-build /out/final-scrape-fixture /final-scrape-fixture
COPY --from=integration-build /out/faultsuite-fixture /faultsuite-fixture
ENTRYPOINT ["/metricshell"]

FROM scratch AS runtime
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="MetricShell" \
      org.opencontainers.image.description="Bounded local metric supervisor for finite container workloads" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"
USER 65532:65532
COPY --from=build /out/metricshell /metricshell
ENTRYPOINT ["/metricshell"]
