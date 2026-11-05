# Production deployment

[Russian version](../../docs-ru/07-delivery/production-deployment.md)

This is the operational entry point for MetricShell 0.2.2. MetricShell runs as PID 1, owns one workload process tree,
accepts instrumentation locally and exposes `GET /metrics` for Prometheus. The workload never needs a monitoring
destination: Prometheus discovery and scrape configuration belong to infrastructure.

## Install

Use Linux amd64 or arm64, `curl`, Python 3, and OpenSSL 3 with Ed25519 support. The release's
[VERIFYING.md](https://github.com/Denki77/metricshell/releases/download/v0.2.2/VERIFYING.md)
describes the offline evidence checks. The executable procedure below implements its signature and binary checks.
For the complete evidence audit, download all release files before running `sha256sum -c SHA256SUMS`.

First obtain the release public key from the maintainer through an independently authenticated channel
(for example, a previously verified signed message or a direct channel whose identity you already verified).
Confirm the full 32-byte Ed25519 key, then export `TRUSTED_RELEASE_PUBLIC_KEY_HEX` with its 64 hexadecimal characters.
A key downloaded from the same release is only a candidate; neither that key nor its same-origin checksum
establishes independent trust. If no independent key confirmation is available, stop before installation.

```sh
set -eu
version=0.2.2
revision=4b19f7e20b2aab366c5cb69edd9e831bc7c89d58
case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "Unsupported architecture" >&2; exit 1 ;;
esac
: "${TRUSTED_RELEASE_PUBLIC_KEY_HEX:?Set the independently confirmed Ed25519 public key}"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"
base="https://github.com/Denki77/metricshell/releases/download/v$version"
asset="metricshell-linux-$arch"
for file in "$asset" SHA256SUMS SHA256SUMS.sig RELEASE_PUBLIC_KEY VERIFYING.md; do
  curl --fail --location --show-error --connect-timeout 10 --max-time 120 \
    "$base/$file" --output "$file"
done
python3 - <<'VERIFY_KEY'
import os
from pathlib import Path
trusted = bytes.fromhex(os.environ['TRUSTED_RELEASE_PUBLIC_KEY_HEX'])
published = bytes.fromhex(Path('RELEASE_PUBLIC_KEY').read_text().strip())
if len(trusted) != 32 or trusted != published:
    raise SystemExit('Release public key does not match independent trust anchor')
# RFC 8410 SubjectPublicKeyInfo for Ed25519, then the raw 32-byte key.
Path('release-key.der').write_bytes(bytes.fromhex('302a300506032b6570032100') + trusted)
signature = bytes.fromhex(Path('SHA256SUMS.sig').read_text().strip())
if len(signature) != 64:
    raise SystemExit('Invalid Ed25519 signature length')
Path('signature.bin').write_bytes(signature)
VERIFY_KEY
openssl pkeyutl -verify -pubin -keyform DER -inkey release-key.der \
  -rawin -in SHA256SUMS -sigfile signature.bin
# Verify exactly the selected asset, without requiring other release files.
python3 - "$asset" <<'VERIFY_HASH'
import hashlib
from pathlib import Path
import sys
asset = sys.argv[1]
lines = [line.split() for line in Path('SHA256SUMS').read_text().splitlines()]
selected = [fields[0] for fields in lines if len(fields) == 2 and fields[1] == asset]
if len(selected) != 1 or hashlib.sha256(Path(asset).read_bytes()).hexdigest() != selected[0]:
    raise SystemExit('Missing, duplicate or mismatched binary checksum')
VERIFY_HASH
chmod 0755 "$asset"
test "$(./"$asset" --version)" = "metricshell version=$version revision=$revision"
sudo install -m 0755 "$asset" /usr/local/bin/metricshell
metricshell --version
```

Tags `ghcr.io/denki77/metricshell:0.2.2`, `metricshell-artifact:0.2.2` and `metricshell-example:0.2.2`
are discovery references for `linux/amd64` and `linux/arm64`. Use immutable digests in production:

```sh
docker pull ghcr.io/denki77/metricshell@sha256:20d5b0cec5195d677f49b65bf315637bf0fed6899a936de527392f85df57f8ec
docker run --rm ghcr.io/denki77/metricshell@sha256:20d5b0cec5195d677f49b65bf315637bf0fed6899a936de527392f85df57f8ec --version
```

The artifact image has the same digest and contains `/metricshell` for multi-stage copying. The runtime image
is minimal and contains no shell; supply your workload in your application image. The published example is
`ghcr.io/denki77/metricshell-example@sha256:56fd77bb5534549a12b1e1a7948a3cd2d63a08de4ee79bb32bbbb8b2a13e0f6b`.
Never use `latest` for production.

For Kubernetes use the release asset `metricshell-kubernetes-v0.2.2.tar.gz`, which already contains the example
digest. The checked-in Job/CronJob/Deployment files in `implementation/examples/kubernetes/production` are templates.
In v0.2.2, `OCI_DIGESTS`, `METADATA` and the Kubernetes bundle are outside the Ed25519-signed manifest;
see the [artifact audit](releases/v0.2.2-audit.md) before relying on them. This is not retroactively repaired.

Reproduce anonymous consumer acceptance from the repository (Python 3, Docker, native/QEMU support for both architectures):

```sh
make -C implementation published-release-smoke
```

The test uses isolated credentials, exact published digests, both architectures, bounded waits and cleanup.
It never builds MetricShell from local source. Private package visibility is a failing result requiring the owner
to change the relevant GHCR package to Public; authentication is not a workaround.

## Runtime contract

MetricShell must be PID 1 and the workload follows the standalone `--`. It forwards signals to the workload process
group, reaps descendants and preserves workload exit status. Exposition defaults to port 9090. Snapshot and Managed
ingestion stay private: mount a writable directory for Unix sockets (normally `/run/metricshell`) or an atomic snapshot
file; the root filesystem can remain read-only. No Kubernetes API or service-account token is required.

Prometheus scrapes `GET /metrics`; `/healthz` and `/readyz` are probes and do not count as final scrapes. Use service
discovery, annotations or a PodMonitor. Choose a scrape timeout below the interval. Finite workloads should use final
wait mode `scrapes`, a finite timeout and enough pod lifetime for at least one eligible scrape.

## Security baseline

Run as a fixed non-root UID/GID, use a read-only root filesystem, drop all capabilities, set
`allowPrivilegeEscalation: false`, `no-new-privileges`, and `seccompProfile.type: RuntimeDefault`. Disable service-account
token mounting. Expose only port 9090. Keep the Managed Unix socket in a mode-0700 directory shared only with the
workload. Do not publish the HTTP ingestion listener; ingestion has no reason to be reachable from the network.

## Resources and lifecycle

Start with the requests/limits in the supplied manifests and tune from measurements. Bound memory, PIDs and `nofile`;
also configure snapshot/decoded byte limits, scrape concurrency, Managed connections, queue capacity, descriptors,
series, label bytes and histogram buckets. Rejection is preferable to unbounded growth.

`terminationGracePeriodSeconds` must exceed MetricShell's total shutdown grace. The supplied manifests use 32 seconds
outside and 30 seconds inside, reserving two seconds for cleanup. Workload shutdown, Managed admission close/drain,
freeze, final Core installation and final-scrape wait are all bounded. If Prometheus is unavailable, timeout ends the
wait and the original workload result is preserved.

## Local instrumentation

Managed clients default to `/run/metricshell/managed.sock`, so shell workloads can declare and update metrics without
passing a destination:

```sh
metricshell managed declare jobs counter "Processed jobs" worker
metricshell managed counter-initialize jobs 0 worker=batch
metricshell managed counter-add jobs 1 worker=batch
metricshell managed declare queue_depth gauge "Queued jobs"
metricshell managed gauge-set queue_depth 7
metricshell managed declare job_seconds histogram "Job duration" - 0.1 1 +Inf
metricshell managed histogram-observe job_seconds 0.42
```

PHP 5.4 copy/paste usage is in `implementation/examples/clients/php54`. Both clients know only the local Unix socket.

## Troubleshooting

- Workload did not start: inspect `workload.start_failed`, executable path, permissions and exit 73.
- Malformed metrics or Core candidate rejected: inspect the bounded rejection reason; the last valid snapshot remains.
- Resource rejection or overload: inspect self-metrics, then raise the specific bound only after measuring demand.
- Final scrape timeout: verify discovery, target readiness, scrape interval and timeout; the wait is intentionally finite.
- Bind failure: ensure exposition and ingestion addresses do not collide and the socket directory is writable/private.
- Managed `UNKNOWN`: the request may have committed after client acknowledgement was lost; reconcile deliberately.
- Prometheus does not discover the target: check labels/annotations or PodMonitor selectors and network policy to 9090.
- Restart: a new Managed process starts an empty epoch by design; MetricShell does not persist application metrics.

MetricShell correctness guarantees are defined by the specifications and tests. Resource limits are hard bounds;
benchmark observations are not an SLO.
