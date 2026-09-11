# Pinned multi-stage copy

Release examples must copy MetricShell from an immutable artifact image:

```dockerfile
COPY --from=ghcr.io/denki77/metricshell-artifact@sha256:<immutable-digest> /metricshell /usr/local/bin/metricshell
```

Mutable tags alone are not valid release evidence. Release verification checks static `linux/amd64` and `linux/arm64`
binaries, `SHA256SUMS`, version output, and OCI `org.opencontainers.image.version` / `revision` labels.
