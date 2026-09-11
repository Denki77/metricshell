# Hardened runtime example

MetricShell does not require root privileges or Linux capabilities. This example runs the runtime image as an arbitrary
non-root UID/GID, mounts only `/run/metricshell` as writable, keeps the root filesystem read-only, and grants producer
access through the configured runtime group.

Required runtime posture:

- `user: "65532:65532"` or another application-owned non-root UID/GID;
- `read_only: true`;
- `cap_drop: ["ALL"]`;
- `security_opt: ["no-new-privileges:true"]`;
- `pids_limit: 64`, `mem_limit: 64m`, `nofile: 64/64`;
- no published ingestion port; HTTP ingestion, when selected, remains loopback-only.
