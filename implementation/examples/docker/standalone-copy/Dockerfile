FROM alpine:3.22 AS application
ARG TARGETARCH
COPY release/metricshell-linux-${TARGETARCH} /tmp/metricshell-linux-${TARGETARCH}
COPY release/SHA256SUMS /tmp/SHA256SUMS
RUN cd /tmp && grep "metricshell-linux-${TARGETARCH}$" SHA256SUMS | sha256sum -c - && \
    install -m 0755 "metricshell-linux-${TARGETARCH}" /usr/local/bin/metricshell
COPY workload.sh /usr/local/bin/example-workload
RUN chmod 0755 /usr/local/bin/example-workload && mkdir -p /run/metricshell && \
    chown -R 65532:65532 /run/metricshell && chmod 0700 /run/metricshell
USER 65532:65532
EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/metricshell", "--mode=managed-registry", "--exposition-listen=0.0.0.0:9090", "--"]
CMD ["/usr/local/bin/example-workload"]
