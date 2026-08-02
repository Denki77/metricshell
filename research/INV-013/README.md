# INV-013 — Distribution Models

**Status:** in progress
**macOS reference run:** `results/20260803T103140Z`
**Ubuntu/LinuxKit run:** pending
**Report:** [report.md](report.md)

## Question

How should MetricShell be added to application images?

## Context and Hypotheses

Distribution must not change the runtime or ADR-004 metric contract. The same versioned static executable should work as
a standalone copied artifact, a multi-stage build input, or a small base image. A language-specific convenience image
may reduce onboarding work but increases image size, maintenance and application coupling.

The hypothesis is that a checksummed standalone static binary plus a documented multi-stage copy is the smallest
sufficient default. A base image is usable but restricts the application's base-image freedom; convenience images
should not become the primary distribution model.

## Evidence Required

- standalone artifact export through BuildKit local output and SHA-256 verification, without creating a stopped
  container;
- scratch, Alpine/musl and PHP Alpine execution;
- non-root base and multi-stage images;
- version pinning through binary output and OCI labels;
- linux/amd64 and linux/arm64 builds and execution;
- reproducible byte-identical rebuild;
- image-size comparison and SPDX evidence;
- immutable multi-platform digests for every base and helper image;
- automatic pinned binfmt/QEMU registration for cross-architecture execution on plain Docker Engine;
- matching-fingerprint Ubuntu repeat.

## Current Result

The macOS Docker Desktop/LinuxKit run passed 24/24 assertions. The same CGO-free artifact ran in scratch, Alpine and PHP
Alpine. Both amd64 and arm64 images were built and executed, including amd64 emulation on the arm64 host. A no-cache
rebuild produced the same artifact SHA-256
`f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486`.

All external images are pinned to immutable multi-platform manifest digests: Go
`sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f`, PHP
`sha256:afdf8b1fee58486ccc0dab5f30f634b86873d56dac985f71ba217945647c05ad`, and Alpine
`sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc`, and binfmt/QEMU
`sha256:1b804311fe87047a4c96d38b4b3ef6f62fca8cd125265917a9e3dc3c996c39e6`. The selected platform image IDs are recorded
in `base-images.tsv`.

Measured image sizes were 1,507,480 bytes for scratch, 10,333,271 bytes for the Alpine base candidate and 97,875,364
bytes for the PHP multi-stage application example. The investigation remains in progress pending Ubuntu confirmation.

## Provisional Admissible Values

- primary artifact: CGO-free static linux/amd64 and linux/arm64 executable;
- pinning: explicit version in binary and OCI labels plus published SHA-256;
- default integration: copy a verified artifact or copy it from a pinned multi-stage build;
- runtime user: non-root in supplied examples;
- base image: optional convenience, never required by MetricShell core;
- language convenience image: example only, not the default release unit;
- supply-chain evidence: checksum, OCI metadata and SPDX document for every release artifact;
- base identity: every external build and helper image must use an immutable multi-platform digest;
- reproducibility: the validated claim is byte-identical rebuild within the same pinned toolchain/daemon environment,
  not full independent supply-chain reproducibility.

## Running the Prototype

```bash
./research/INV-013/run-bench.sh
latest="$(cat research/INV-013/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/architectures.tsv"
cat "$latest/observations.tsv"
cat "$latest/base-images.tsv"
cat "$latest/artifacts/SHA256SUMS"
cat "$latest/sbom.spdx"
cat "$latest/environment.tsv"
cat "$latest/run-summary.tsv"
```

The same command is used on macOS and Ubuntu. Compare `benchmark_code_fingerprint_sha256`; result files and documents
are excluded from the fingerprint. Do not prepare emulation manually: the runner uses a digest-pinned binfmt image to
register amd64 and arm64 before the cross-architecture cases. The Docker account must be allowed to start this
privileged helper container.

A successful run takes time to pull and build both architectures. A fast exit is a failure, not an empty result. The
runner immediately updates `latest-results.txt` and records the failed phase in both `summary.tsv` and
`run-summary.tsv`. It also prints a normalized reason without host paths. Before running, `docker info` must succeed as
the current user without `sudo`. Inspect the retained evidence with:

```bash
latest="$(cat research/INV-013/latest-results.txt)"
cat "$latest/run-summary.tsv"
cat "$latest/failure.tsv"
cat "$latest/docker-info.log"
cat "$latest/binfmt-install.log"
cat "$latest/amd64.build.log"
cat "$latest/arm64.build.log"
```

## Prototype Limits

- Research executable, not a release pipeline or signing service.
- Artifact extraction requires Docker Buildx with the local exporter; the preflight verifies that Buildx is available.
- Static-link validation runs inside the already built Alpine candidate and does not require a host bind mount.
- SPDX contains the primary artifact identity; transitive OS/package inventories need a production SBOM generator.
- OCI signature, transparency-log publication and vulnerability scanning are not implemented.
- Only scratch, Alpine and PHP Alpine were executed; distroless, Debian/glibc and Windows are outside the selected Linux
  static-binary scope.
- Image sizes depend on pinned upstream bases and are observations, not fixed limits.
- Cross-architecture execution registers binfmt through a privileged helper container; it changes the Docker host/VM
  emulation registration and therefore requires the corresponding Docker permission.
- The byte-identical rebuild shares one Docker daemon and pinned toolchain environment. Ubuntu matching will confirm
  portability of this benchmark, not by itself prove independent-builder or full supply-chain reproducibility.

## Additional Benchmarks

| Benchmark                                        | Status                              |
|--------------------------------------------------|-------------------------------------|
| standalone binary extraction/checksum            | covered                             |
| scratch, Alpine and PHP Alpine execution         | covered                             |
| non-root base and application image              | covered                             |
| amd64 and arm64 build/execution                  | covered                             |
| explicit version output and OCI label            | covered                             |
| byte-identical no-cache rebuild                  | covered                             |
| immutable base/helper image digests              | covered                             |
| pinned binfmt/QEMU setup for plain Ubuntu Docker | covered                             |
| image sizes                                      | covered                             |
| SPDX artifact record                             | covered                             |
| matching Ubuntu fingerprint                      | pending                             |
| cosign/SLSA provenance and registry verification | recommended for release engineering |
| vulnerability scan of final release bases        | recommended at release time         |

## Better Follow-up Benchmarking

Run the unchanged fingerprint on Ubuntu and require the same immutable image references from `base-images.tsv`, then
attach registry-published multi-architecture manifests, signatures, provenance and a full generated SBOM. Rebuild in
an independent clean CI worker before making an independent-builder or full supply-chain reproducibility claim.

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS evidence: `results/20260803T103140Z/`
- Ubuntu evidence: pending
- Detailed report: [report.md](report.md)
- ADR: pending
