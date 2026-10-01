# Transport conformance

This profile publishes `fixtures/application.snapshot`, malformed input and `zero-series.snapshot` through every stable
transport. The runner strips `metricshell_*` self-metrics and requires byte-identical application exposition, retention
of the last valid snapshot after rejection, and complete removal after the zero-series snapshot.
