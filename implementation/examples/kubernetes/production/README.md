# Kubernetes production templates

These checked-in files are `TEMPLATE — NOT DIRECTLY DEPLOYABLE`: the release workflow replaces the image token with the
immutable digest of the published example image and attaches the rendered bundle to the GitHub Release. Apply only that
rendered bundle. MetricShell is PID 1 in the image and owns the workload. Only exposition port 9090 is network-visible;
the Managed socket remains in a private `emptyDir`. Prometheus may use the annotations or `podmonitor.yaml`.

The Job and CronJob retain final state for a bounded eligible scrape; the Deployment publishes coalesced updates while
the worker runs. Restart creates a fresh empty Managed epoch. None require Kubernetes API access or a service account.
