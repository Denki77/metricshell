ARG METRICSHELL_ARTIFACT_REF=ghcr.io/denki77/metricshell-artifact@sha256:__METRICSHELL_IMAGE_DIGEST__
FROM ${METRICSHELL_ARTIFACT_REF} AS metricshell
FROM alpine:3.22
COPY --from=metricshell /metricshell /usr/local/bin/metricshell
COPY workload.sh /usr/local/bin/example-workload
RUN chmod 0755 /usr/local/bin/example-workload && mkdir -p /run/metricshell && \
    chown -R 65532:65532 /run/metricshell && chmod 0700 /run/metricshell
USER 65532:65532
EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/metricshell", "--mode=managed-registry", "--exposition-listen=0.0.0.0:9090", "--"]
CMD ["/usr/local/bin/example-workload"]
