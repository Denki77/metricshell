# INV-013 Report — Distribution Models

**Status:** in progress
**Run date:** 2026-08-03
**Reference run:** `results/20260803T103140Z`
**Docker platform:** 29.6.2, LinuxKit aarch64
**Fingerprint:** `01b04f50305cce6474d74f9fa196d35bcd60dd65281134056d568cb2d4e96ea4`

## Goal and Scope

Compare standalone, multi-stage, base-image and language-convenience distribution without changing MetricShell behavior
or ADR-004 complete-snapshot semantics. The prototype is a versioned process identity executable; metric processing is
intentionally not duplicated in this packaging investigation.

## Prototype

- `cmd/inv013` emits version, OS, architecture, UID and GID;
- `Dockerfile.binary` creates a scratch artifact image;
- `Dockerfile.base` creates a non-root Alpine base candidate;
- `Dockerfile.multistage` demonstrates a non-root PHP Alpine application image;
- `run-bench.sh` registers pinned binfmt/QEMU, builds, executes and cross-builds candidates, exports the standalone
  filesystem directly through BuildKit, hashes it and performs a no-cache export for comparison.

## Environment and Results

| Candidate          |         Size | Runtime        | User                           | Result |
|--------------------|-------------:|----------------|--------------------------------|--------|
| standalone/scratch |  1,507,480 B | no libc        | root in scratch research image | pass   |
| Alpine base        | 10,333,271 B | musl userspace | non-root                       | pass   |
| PHP multi-stage    | 97,875,364 B | PHP 8.3 Alpine | non-root                       | pass   |

All 24 assertions passed. The artifact ran as arm64 natively and amd64 through Docker emulation. Image metadata reported
the correct architecture. Every candidate returned version `0.13.0-research`. The rebuilt artifact exactly matched
SHA-256 `f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486`.

Every external image, including the binfmt/QEMU helper, is pinned by immutable multi-platform digest. `base-images.tsv`
also records the native arm64 image IDs selected from those manifests, so Ubuntu comparison can verify both the same
index identity and its x86_64 platform selection. The runner registers both benchmark architectures itself so a plain
Ubuntu Docker Engine does not depend on manually prepared emulation.

The static-link check executes `ldd` inside the built Alpine candidate. It does not mount the exported host artifact;
the exact exported artifact is independently proven executable in the scratch candidate, where no dynamic loader exists.

## Hypothesis Evaluation

### Static artifact plus multi-stage copy is sufficient

Provisionally supported. One CGO-free binary worked without libc and inside both tested musl images. Applications keep
their own base and dependency policy.

### A MetricShell base image is operationally useful

Supported as optional convenience, not as a mandatory integration. It supplies a non-root default but adds an Alpine
base and constrains downstream `FROM` selection.

### Language-specific convenience images should be primary

Not supported. The PHP example was roughly 65 times the scratch candidate size and couples MetricShell release work to a
language runtime lifecycle without changing core functionality.

## Evaluation Criteria

| Criterion             | Result                                                            |
|-----------------------|-------------------------------------------------------------------|
| image size            | scratch is smallest; base/convenience add expected runtime layers |
| libc compatibility    | static artifact ran in scratch and musl images                    |
| amd64/arm64           | both built and executed                                           |
| non-root              | base and multi-stage examples passed                              |
| version pinning       | binary and OCI labels agree                                       |
| reproducibility       | byte-identical rebuild in one pinned toolchain environment passed |
| supply-chain identity | immutable image digests, artifact SHA-256 and SPDX recorded       |
| application freedom   | multi-stage copy preserves chosen application base                |

## Provisional Policy

Publish checksummed static binaries and a multi-architecture artifact/image. Document a pinned multi-stage copy as the
default container integration. Keep a minimal base image optional. Do not make language-specific images a required
distribution surface. Release engineering must add cryptographic signatures, full SBOM and provenance.

## Limitations and Additional Benchmarking

The macOS run uses LinuxKit; Ubuntu is pending. Although the toolchain and helper images are immutable, the local
rebuild
still shares one Docker daemon/toolchain environment. A matching Ubuntu fingerprint proves the portable benchmark with
the same declared inputs; it does not by itself prove full reproducible-build supply-chain behavior. Registry manifest
publication, cosign, SLSA provenance, an independent clean-builder rebuild and CVE scanning remain release-pipeline
work. The complete local matrix—artifact copy, pinned images, three runtimes, both architectures, non-root, versioning,
sizes, checksum, rebuild and SPDX—was executed and is not delegated.

## Conclusion

The evidence provisionally favors the standalone static artifact and pinned multi-stage copy. The base image is an
optional convenience; language-specific images do not justify primary status. Final completion and ADR remain blocked
only on the matching Ubuntu run and release-supply-chain validation.

## Decision Output

- Raw evidence: `results/20260803T103140Z/`
- Artifact SHA-256: `f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486`
- Ubuntu evidence: pending
- ADR: pending
