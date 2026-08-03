# INV-013 Report — Distribution Models

**Status:** completed
**Run date:** 2026-08-03
**Reference runs:** `results/20260803T103140Z`, `results/20260803T135959Z`
**Fingerprint:** `01b04f50305cce6474d74f9fa196d35bcd60dd65281134056d568cb2d4e96ea4`
**Decision:** [ADR-013](../../docs/06-architecture/adr/ADR-013.md)

## Goal and Scope

Compare standalone, multi-stage, base-image and language-convenience distribution without changing MetricShell runtime
behavior or ADR-004 complete-snapshot semantics. Packaging must preserve one versioned executable and must not introduce
a second metrics implementation.

## Prototype

- `cmd/inv013` reports version, OS, architecture, UID and GID;
- `Dockerfile.binary` produces a scratch artifact image;
- `Dockerfile.base` produces a non-root Alpine base candidate;
- `Dockerfile.multistage` demonstrates a non-root PHP Alpine application image;
- `run-bench.sh` registers pinned binfmt/QEMU, builds and executes both architectures, exports the standalone artifact
  through BuildKit local output, checks its hash and performs a no-cache rebuild comparison.

All external build and helper images use immutable multi-platform manifest digests. Artifact extraction does not create
a stopped container and static-link checking executes inside the already built Alpine candidate.

## Run Environments

| Environment                       | Date       | Docker | Architecture | Result                     | Status                |
|-----------------------------------|------------|-------:|--------------|----------------------------|-----------------------|
| Docker Desktop on macOS/LinuxKit  | 2026-08-03 | 29.6.2 | aarch64      | `results/20260803T103140Z` | 24/24 assertions pass |
| Docker Desktop on Ubuntu/LinuxKit | 2026-08-03 | 27.4.0 | x86_64       | `results/20260803T135959Z` | 24/24 assertions pass |

Both runs have the same benchmark fingerprint shown above. Every assertion passed in both environments. Both container
environments use LinuxKit; the comparison covers LinuxKit aarch64 and x86_64, not native non-LinuxKit Linux.

## Results

### Runtime and architecture matrix

The same CGO-free artifact executed in scratch, Alpine/musl and PHP Alpine. Both `linux/amd64` and `linux/arm64` images
were built and executed in each run, using digest-pinned binfmt/QEMU where emulation was needed. Every candidate
reported
version `0.13.0-research`, and OCI architecture metadata matched the requested platform.

### Artifact identity and rebuild

The exported artifact and no-cache rebuild were byte-identical within each pinned Docker/toolchain environment. The
artifact SHA-256 was:

```text
f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486
```

This establishes deterministic rebuild behavior for the tested pinned environment. It is not a claim that independent
builders or the complete release supply chain are reproducible.

### Image-size observations

| Candidate          | macOS/LinuxKit aarch64 | Ubuntu/LinuxKit x86_64 | Runtime        | User          |
|--------------------|-----------------------:|-----------------------:|----------------|---------------|
| standalone/scratch |            1,507,480 B |              678,173 B | no libc        | research root |
| Alpine base        |           10,333,271 B |            4,310,805 B | musl userspace | non-root      |
| PHP multi-stage    |           97,875,364 B |           38,301,971 B | PHP 8.3 Alpine | non-root      |

The architecture-dependent sizes are observations of the pinned platform variants, not universal release-size limits.
The standalone candidate remained the smallest in both environments.

### Immutable inputs

The Go, PHP, Alpine and binfmt/QEMU references were identical immutable multi-platform digests in both runs.
`base-images.tsv` records the selected native platform image ID separately, so an expected arm64/x86_64 platform-ID
difference is not confused with mutable-tag drift.

### Supply-chain evidence

The runner retained the artifact checksum, OCI version metadata and an SPDX artifact record. Registry publication,
signatures, transparency-log entries, SLSA provenance, full package inventory and vulnerability scanning are release
pipeline responsibilities and were not fabricated by this prototype.

## Hypothesis Evaluation

### Static artifact plus multi-stage copy is sufficient

Confirmed. One static executable worked without libc and inside both tested musl-based application environments on both
architectures. A pinned multi-stage copy preserves the application's base-image and dependency choices.

### A MetricShell base image is useful

Confirmed as an optional convenience. It supplies a non-root default but constrains the downstream `FROM` choice and
adds runtime layers, so it is not required by the core distribution contract.

### Language-specific convenience images should be primary

Rejected. The PHP example was materially larger than the scratch artifact in both architectures and couples
MetricShell releases to a language-runtime lifecycle without adding core behavior.

## Accepted Policy

- Publish checksummed static `linux/amd64` and `linux/arm64` artifacts.
- Expose explicit version output and matching OCI labels.
- Document verified artifact copy and pinned multi-stage copy as the default container integrations.
- Keep a minimal non-root base image optional.
- Treat language-specific images as examples, not required release units.
- Pin every build and helper image by immutable multi-platform digest.
- Publish checksum, OCI metadata, SBOM, signatures and provenance for production releases.
- Describe byte-identical local rebuild and independent supply-chain reproducibility as separate claims.

## Limitations

- Both container environments use LinuxKit; native non-LinuxKit Linux was not tested.
- The prototype is not a registry, signing service or complete release pipeline.
- SPDX evidence identifies the primary artifact but is not a full transitive package inventory.
- Scratch, Alpine and PHP Alpine were tested; distroless, Debian/glibc and Windows were outside the selected scope.
- Cross-architecture execution requires permission to run a privileged binfmt helper.
- Image sizes depend on the pinned architecture-specific upstream layers.

## Additional Benchmarking

| Benchmark                                   | Status    | Evidence/Boundary                             |
|---------------------------------------------|-----------|-----------------------------------------------|
| standalone artifact extraction and checksum | covered   | artifact directory and `SHA256SUMS`           |
| scratch, Alpine and PHP Alpine execution    | covered   | assertions and per-image logs                 |
| non-root base and application images        | covered   | UID/GID assertions                            |
| amd64 and arm64 build/execution             | covered   | `architectures.tsv`                           |
| immutable base/helper digests               | covered   | `base-images.tsv`                             |
| byte-identical local no-cache rebuild       | covered   | artifact hash assertion                       |
| image-size comparison                       | covered   | `observations.tsv` in both environments       |
| matching Ubuntu fingerprint                 | covered   | 24/24 assertions, identical fingerprint       |
| independent clean-builder rebuild           | follow-up | required for a stronger reproducibility claim |
| cosign/SLSA/full SBOM/CVE scan              | follow-up | production release engineering                |

## Conclusion

INV-013 is confirmed by matching-fingerprint macOS/LinuxKit and Ubuntu/LinuxKit runs. The primary distribution model is
a checksummed static artifact with a pinned multi-stage copy. A minimal base image is optional, and language-specific
images are examples rather than primary release surfaces. The decision is recorded in
[ADR-013](../../docs/06-architecture/adr/ADR-013.md).

## Decision Output

- macOS evidence: `results/20260803T103140Z/`
- Ubuntu evidence: `results/20260803T135959Z/`
- artifact SHA-256: `f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486`
- ADR: [ADR-013](../../docs/06-architecture/adr/ADR-013.md)
