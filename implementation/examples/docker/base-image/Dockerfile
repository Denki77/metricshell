ARG METRICSHELL_IMAGE_REF=ghcr.io/denki77/metricshell@sha256:20d5b0cec5195d677f49b65bf315637bf0fed6899a936de527392f85df57f8ec
FROM ${METRICSHELL_IMAGE_REF} AS metricshell
FROM alpine:3.22
COPY --from=metricshell /metricshell /usr/local/bin/metricshell
COPY workload.sh /usr/local/bin/example-workload
RUN chmod 0755 /usr/local/bin/example-workload && mkdir -p /run/metricshell && \
    chown -R 65532:65532 /run/metricshell && chmod 0700 /run/metricshell
USER 65532:65532
EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/metricshell", "--mode=managed-registry", "--exposition-listen=0.0.0.0:9090", "--"]
CMD ["/usr/local/bin/example-workload"]
