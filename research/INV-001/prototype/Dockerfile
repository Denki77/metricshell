FROM golang:1.25

WORKDIR /prototype
COPY go.mod ./
COPY cmd ./cmd
COPY workloads ./workloads
RUN go build -o /usr/local/bin/metricshell ./cmd/metricshell \
    && go build -o /usr/local/bin/workload-trap ./cmd/workload-trap \
    && go build -o /usr/local/bin/workload-cpu ./cmd/workload-cpu \
    && go build -o /usr/local/bin/workload-stubborn ./cmd/workload-stubborn \
    && chmod +x /prototype/workloads/*.sh

EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/metricshell"]
