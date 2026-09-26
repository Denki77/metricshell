# PHP 5.4 Managed Aggregation client

`metricshell.php` is a stateless protocol-v1 reference client. It stores no registry and opens one Unix connection per
operation. Exit codes are `0` accepted, `2` local invocation, `3` rejected, `4` overload, `5` protocol, `6` definite
transport/connect failure, and `7` unknown.

An `unknown` result means the complete request may have committed before the response was lost. Do not retry it blindly.
Reconnect requires no local registry reconstruction.
