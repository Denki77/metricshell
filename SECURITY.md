# Security Policy

## Supported versions

MetricShell is currently pre-1.0. Security fixes target the latest development line unless a release branch is
explicitly announced.

## Reporting a vulnerability

Please do not open public issues for suspected vulnerabilities. Use GitHub private vulnerability reporting when
available for this repository, or contact the maintainer through a private channel.

Include:

- affected version or commit;
- a minimal reproduction;
- expected impact;
- whether the issue requires a malicious workload, malicious metrics publisher or malicious artifact producer.

## Supply chain

Release evidence is generated in Docker. Signing keys are expected to live outside the repository and be supplied to
release builds as BuildKit secrets.
