# PHP 5.4 Managed Aggregation client

`metricshell.php` is a stateless protocol-v1 reference client. It stores no registry and opens one Unix connection per
operation. Exit codes are `0` accepted, `2` local invocation, `3` rejected, `4` overload, `5` protocol, `6` definite
transport/connect failure, and `7` unknown.

An `unknown` result means the complete request may have committed before the response was lost. Do not retry it blindly.
The same rule applies when a request context is cancelled after owner admission: MetricShell cannot prove whether the
operation committed. Protocol v1 has no idempotency key or exactly-once retry semantics; in particular, blindly
retrying `counter_add 1` can increment twice.
Reconnect requires no local registry reconstruction.

Accepted means that the registry mutation committed; Core installs the authoritative complete generation during
finalization. A rejected operation preserves the committed registry and active Core state. Resource, semantic,
overload, protocol and late responses are definite failures and must be handled by exit code. Only transport loss after
complete submission is `unknown`, and blindly retrying it can apply a non-idempotent mutation twice.

```sh
php metricshell.php /run/metricshell/managed.sock 1 declare jobs counter "Jobs processed." worker
php metricshell.php /run/metricshell/managed.sock 1 counter-add jobs 1 worker=batch
```
