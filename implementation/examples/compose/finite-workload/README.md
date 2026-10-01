# Finite workload final scrape

The workload freezes its final registry until Prometheus completes an eligible `/metrics` response or the bounded
15-second timeout expires. The verifier queries Prometheus while the target remains in final wait. Run through
`../test-examples.sh`; direct teardown may remove the target before evidence is recorded.
