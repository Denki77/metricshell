# Runtime-image copy

This example copies MetricShell from the official non-root runtime image while keeping the application's Alpine
userspace. The Dockerfile defaults to the real immutable v0.2.2 production baseline digest; it can be built as checked in.

```sh
docker build --check implementation/examples/docker/base-image
docker build -t metricshell-example:local implementation/examples/docker/base-image
```

A future separately approved release supplies its newly published immutable runtime reference with
`--build-arg METRICSHELL_IMAGE_REF`. The release workflow always passes that digest explicitly. An explicitly empty
or invalid reference fails Buildx checks; no floating production default or warning suppression is used.

```sh
docker build --check --build-arg METRICSHELL_IMAGE_REF= implementation/examples/docker/base-image
# Expected: nonzero exit for the deliberately empty reference.
```

The existing example gate also builds against its local runtime candidate; that local validation reference is never
used for published production images.
