#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"&&pwd)"; REPO_DIR="$(git -C "$ROOT_DIR" rev-parse --show-toplevel)"; PROTO="$ROOT_DIR/prototype"
RUN_ID="${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"; RESULTS_DIR="$ROOT_DIR/results/$RUN_ID"; RESULTS_REF="research/INV-012/results/$RUN_ID"; mkdir -p "$RESULTS_DIR"
ASSERTIONS="$RESULTS_DIR/assertions.tsv"; OBSERVATIONS="$RESULTS_DIR/observations.tsv"; SUMMARY="$RESULTS_DIR/summary.tsv"; PROFILE="${INV012_PROFILE:-metricshell-inv012}"; CTX="$PROFILE"; IMAGE=metricshell-inv012:research; CHART_VERSION=88.1.2
MINIKUBE_VERSION=v1.38.1; KUBERNETES_VERSION=v1.34.0; MINIKUBE_START_TIMEOUT_SECONDS=900; MINIKUBE_BIN=
printf 'case\tassertion\texpected\tactual\tresult\n' >"$ASSERTIONS"; printf 'case\tmetric\tvalue\tunit\tnote\n' >"$OBSERVATIONS"; printf 'case\tresult\tdetails\n' >"$SUMMARY"
REPLICA_EVIDENCE="$RESULTS_DIR/prometheus-replica-evidence.tsv"; printf 'replica\tpod\tevaluation_time_unix\tquery_status\tseries_present\tsample_value\n' >"$REPLICA_EVIDENCE"
printf '%s\n' "$RESULTS_REF" >"$ROOT_DIR/latest-results.txt"
CURRENT_PHASE=initialization
ERROR_DETAIL=
redact_path(){ sed -E "s#$REPO_DIR#<repo>#g; s#$ROOT_DIR#<inv012>#g; s#unix:///[^[:space:];,]+#unix://<socket>#g; s#Path:[[:space:]]+/[^[:space:]]+#Path: <redacted>#g; s#(^|[[:space:]])/(Users|home|private|tmp|var)/[^[:space:];,]+#\\1<path>#g"; }
normalize_preflight_error(){
 local label="$1" log="$2" raw
 raw="$(cat "$log")"
 if printf '%s' "$raw"|grep -Eqi 'permission denied|access denied'; then
  ERROR_DETAIL="Docker daemon access denied; docker info must succeed for the current user"
 elif printf '%s' "$raw"|grep -Eqi 'failed to connect|cannot connect|daemon is not running|no such file or directory'; then
  ERROR_DETAIL="Docker daemon is unreachable; start Docker and verify docker info as the current user"
 else
  ERROR_DETAIL="$(printf '%s' "$raw"|tail -n 8|tr '\t\n' '  '|redact_path)"
 fi
 [[ -n "$ERROR_DETAIL" ]]||ERROR_DETAIL="$label failed without diagnostic output"
 printf 'preflight_error\t%s\n' "$ERROR_DETAIL" >"$log"
}
hash_file(){ if command -v sha256sum >/dev/null; then sha256sum "$1"|awk '{print $1}'; else shasum -a 256 "$1"|awk '{print $1}'; fi; }; hash_stdin(){ if command -v sha256sum >/dev/null; then sha256sum|awk '{print $1}'; else shasum -a 256|awk '{print $1}'; fi; }
fingerprint(){ { find "$PROTO" -type f|sort; printf '%s\n' "$ROOT_DIR/run-bench.sh"; }|while IFS= read -r f; do printf '%s  %s\n' "$(hash_file "$f")" "${f#$ROOT_DIR/}"; done|hash_stdin; }
assert_eq(){ local c="$1" a="$2" e="$3" v="$4" r=fail; [[ "$e" == "$v" ]]&&r=pass; printf '%s\t%s\t%s\t%s\t%s\n' "$c" "$a" "$e" "$v" "$r" >>"$ASSERTIONS"; }; observe(){ printf '%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" "${5:-}" >>"$OBSERVATIONS"; }
finish_case(){ local c="$1" n; n="$(awk -F '\t' -v c="$c" 'NR>1&&$1==c&&$5!="pass"{n++}END{print n+0}' "$ASSERTIONS")"; [[ "$n" == 0 ]]&&printf '%s\tpass\tall assertions passed\n' "$c" >>"$SUMMARY"||printf '%s\tfail\t%s assertion(s) failed\n' "$c" "$n" >>"$SUMMARY"; }
mk(){ "$MINIKUBE_BIN" "$@"; }
k(){ mk -p "$PROFILE" kubectl -- "$@"; }
now_ms(){ perl -MTime::HiRes=time -e 'printf "%.3f\n",time()*1000'; }
deadline_exec(){
 local seconds="$1"; shift
 perl -MPOSIX=setsid -e '
  use strict; use warnings;
  my $limit = shift @ARGV;
  my $pid = fork(); die "fork failed\n" unless defined $pid;
  if ($pid == 0) { setsid(); exec @ARGV; die "exec failed\n"; }
  my $timed_out = 0;
  $SIG{ALRM} = sub { $timed_out = 1; kill "TERM", -$pid; select undef, undef, undef, 2; kill "KILL", -$pid; };
  alarm $limit; waitpid($pid, 0); alarm 0;
  exit 124 if $timed_out;
  exit(128 + ($? & 127)) if $? & 127;
  exit($? >> 8);
 ' "$seconds" "$@"
}
cleanup(){
 if [[ "${INV012_KEEP_MINIKUBE:-0}" != 1 ]] && [[ -n "$MINIKUBE_BIN" && -x "$MINIKUBE_BIN" ]]; then
  deadline_exec 60 "$MINIKUBE_BIN" delete -p "$PROFILE" >/dev/null 2>&1||docker rm -f "$PROFILE" >/dev/null 2>&1||true
 fi
 [[ -z "$MINIKUBE_BIN" ]]||rm -f "$MINIKUBE_BIN"
}
on_error(){
 local code=$? line="$1" command="$2"
 trap - ERR; set +e
 command="${command//$'\t'/ }"; command="${command//$'\n'/ }"
 command="$(printf '%s' "$command"|redact_path)"; ERROR_DETAIL="$(printf '%s' "${ERROR_DETAIL:-not available}"|redact_path)"
 printf 'error\tfail\tphase=%s; exit_code=%s; detail=%s\n' "$CURRENT_PHASE" "$code" "$ERROR_DETAIL" >>"$SUMMARY"
 printf 'status\terror\nexit_code\t%s\nphase\t%s\nline\t%s\ncommand\t%s\ndetail\t%s\nresults_dir\t%s\n' "$code" "$CURRENT_PHASE" "$line" "$command" "$ERROR_DETAIL" "$RESULTS_REF" >"$RESULTS_DIR/run-summary.tsv"
 printf 'phase\tline\texit_code\tcommand\tdetail\n%s\t%s\t%s\t%s\t%s\n' "$CURRENT_PHASE" "$line" "$code" "$command" "$ERROR_DETAIL" >"$RESULTS_DIR/failure.tsv"
 printf 'INV-012 failed during phase %s (exit %s): %s\nEvidence: %s\n' "$CURRENT_PHASE" "$code" "$ERROR_DETAIL" "$RESULTS_REF" >&2
 exit "$code"
}
require_command(){ command -v "$1" >/dev/null 2>&1 || { ERROR_DETAIL="missing prerequisite: $1"; printf 'INV-012 preflight failed: %s\n' "$ERROR_DETAIL" >&2; return 127; }; }
run_preflight(){
 local label="$1" log="$2" code; shift 2
 if "$@" >"$log" 2>&1; then return 0; else code=$?; fi
 normalize_preflight_error "$label" "$log"
 printf 'INV-012 preflight failed (%s): %s\n' "$label" "$ERROR_DETAIL" >&2
 return "$code"
}
run_step(){
 local label="$1" log="$2" code raw; shift 2
 ERROR_DETAIL=
 if "$@" >"$log" 2>&1; then return 0; else code=$?; fi
 raw="$(tail -n 20 "$log")"
 ERROR_DETAIL="$(printf '%s' "$raw"|tr '\t\n' '  '|redact_path)"
 [[ -n "$ERROR_DETAIL" ]]||ERROR_DETAIL="$label failed without diagnostic output"
 printf 'step_error\t%s\n' "$ERROR_DETAIL" >>"$log"
 printf 'INV-012 step failed (%s): %s\n' "$label" "$ERROR_DETAIL" >&2
 return "$code"
}
run_step_timeout(){
 local label="$1" log="$2" seconds="$3" code raw; shift 3
 ERROR_DETAIL=
 if deadline_exec "$seconds" "$@" >"$log" 2>&1; then return 0; else code=$?; fi
 if [[ "$code" == 124 ]]; then
  ERROR_DETAIL="$label exceeded the stand timeout of ${seconds}s and was terminated"
 else
  raw="$(tail -n 30 "$log")"
  if printf '%s' "$raw"|grep -qi 'kubeadm init timed out'; then
   ERROR_DETAIL="Minikube GUEST_START: kubeadm init timed out"
  else
   ERROR_DETAIL="$(printf '%s' "$raw"|tr '\t\n' '  '|redact_path)"
  fi
 fi
 [[ -n "$ERROR_DETAIL" ]]||ERROR_DETAIL="$label failed without diagnostic output"
 printf 'step_error\t%s\n' "$ERROR_DETAIL" >>"$log"
 printf 'INV-012 step failed (%s): %s\n' "$label" "$ERROR_DETAIL" >&2
 return "$code"
}
fail_with_code(){ return "$1"; }
trap cleanup EXIT
trap 'on_error "$LINENO" "$BASH_COMMAND"' ERR
CURRENT_PHASE=preflight
for required_command in docker helm curl perl git grep mktemp uname; do require_command "$required_command"; done
run_preflight docker "$RESULTS_DIR/docker-info.log" docker info
run_preflight helm "$RESULTS_DIR/helm-version.log" helm version
CURRENT_PHASE=minikube_bootstrap
minikube_os="$(uname -s|tr '[:upper:]' '[:lower:]')"; minikube_machine="$(uname -m)"
case "$minikube_machine" in x86_64) minikube_arch=amd64;; arm64|aarch64) minikube_arch=arm64;; *) ERROR_DETAIL="unsupported Minikube host architecture: $minikube_machine"; false;; esac
case "${minikube_os}-${minikube_arch}" in
 linux-amd64) minikube_sha=099477eaf248bcb5bcea8ce78a2898e93ac01461c35189da1848c3de82ecd22e;;
 linux-arm64) minikube_sha=a0b8a1ebfc8c07a247271d8df98ac0ddd7c8c855b601d402463e2e50c08c6bab;;
 darwin-amd64) minikube_sha=db11dffba835609988e4e98c3a91a38653ce66ddfa8ea3aaea92d87c54a0a348;;
 darwin-arm64) minikube_sha=f9b0c70bb7daf38c683c0b6e46dc1b612600247ae826bf74576807746a919ee8;;
 *) ERROR_DETAIL="unsupported Minikube host platform: ${minikube_os}-${minikube_arch}"; false;;
esac
MINIKUBE_BIN="$(mktemp)"
run_step minikube-download "$RESULTS_DIR/minikube-download.log" curl -fL --retry 3 --connect-timeout 20 -o "$MINIKUBE_BIN" "https://storage.googleapis.com/minikube/releases/${MINIKUBE_VERSION}/minikube-${minikube_os}-${minikube_arch}"
downloaded_sha="$(hash_file "$MINIKUBE_BIN")"
if [[ "$downloaded_sha" != "$minikube_sha" ]]; then ERROR_DETAIL="Minikube checksum mismatch: expected $minikube_sha, got $downloaded_sha"; false; fi
chmod +x "$MINIKUBE_BIN"
run_step minikube-version "$RESULTS_DIR/minikube-version.log" mk version
CURRENT_PHASE=minikube_start
deadline_exec 60 "$MINIKUBE_BIN" delete -p "$PROFILE" >/dev/null 2>&1||docker rm -f "$PROFILE" >/dev/null 2>&1||true
if run_step_timeout minikube-start "$RESULTS_DIR/minikube-start.log" "$MINIKUBE_START_TIMEOUT_SECONDS" "$MINIKUBE_BIN" start -p "$PROFILE" --driver=docker --container-runtime=containerd --cpus=4 --memory=4096 --kubernetes-version="$KUBERNETES_VERSION"; then
 :
else
 start_code=$?; start_detail="$ERROR_DETAIL"
 deadline_exec 120 "$MINIKUBE_BIN" -p "$PROFILE" logs >"$RESULTS_DIR/minikube-cluster.log" 2>&1||true
 redact_path <"$RESULTS_DIR/minikube-cluster.log" >"$RESULTS_DIR/minikube-cluster.redacted.log"; mv "$RESULTS_DIR/minikube-cluster.redacted.log" "$RESULTS_DIR/minikube-cluster.log"
 ERROR_DETAIL="$start_detail"; fail_with_code "$start_code"
fi
CURRENT_PHASE=cluster_readiness
k cluster-info >"$RESULTS_DIR/cluster-info.log" 2>&1; k version --client -o json >"$RESULTS_DIR/kubectl-client-version.log" 2>&1; k get nodes -o wide >"$RESULTS_DIR/nodes.txt" 2>&1
effective_kubectl_version="$(sed -n 's/.*"gitVersion": "\([^"]*\)".*/\1/p' "$RESULTS_DIR/kubectl-client-version.log"|head -1)"
assert_eq toolchain effective_kubectl_version "$KUBERNETES_VERSION" "$effective_kubectl_version"
if command -v kubectl >/dev/null 2>&1; then kubectl version --client -o json >"$RESULTS_DIR/system-kubectl-version.log" 2>&1||true; else printf 'system kubectl is not installed; it is not used by this stand\n' >"$RESULTS_DIR/system-kubectl-version.log"; fi
CURRENT_PHASE=image_build_and_load
docker build --pull=false -t "$IMAGE" "$PROTO" >"$RESULTS_DIR/docker-build.log" 2>&1; docker image save "$IMAGE" -o "$RESULTS_DIR/inv012-image.tar"; mk image load -p "$PROFILE" "$RESULTS_DIR/inv012-image.tar" >"$RESULTS_DIR/image-load.log" 2>&1; rm -f "$RESULTS_DIR/inv012-image.tar"
CURRENT_PHASE=monitoring_install
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update >"$RESULTS_DIR/helm-repo.log" 2>&1; helm repo update >>"$RESULTS_DIR/helm-repo.log" 2>&1
helm upgrade --install inv012-monitoring prometheus-community/kube-prometheus-stack --version "$CHART_VERSION" --kube-context "$CTX" --namespace monitoring --create-namespace -f "$PROTO/k8s/monitoring-values.yaml" --wait --timeout 10m >"$RESULTS_DIR/helm-install.log" 2>&1
k -n monitoring wait --for=condition=Ready pod -l app.kubernetes.io/name=prometheus --timeout=180s >"$RESULTS_DIR/prometheus-ready.log"
replicas="$(k -n monitoring get pods -l app.kubernetes.io/name=prometheus --field-selector=status.phase=Running -o name|wc -l|tr -d ' ')"; assert_eq two_prometheus replicas 2 "$replicas"
k create namespace inv012 >/dev/null
CURRENT_PHASE=experiments

# A real standalone Prometheus with kubernetes_sd_configs role=pod covers direct Pod discovery.
k apply -f - >"$RESULTS_DIR/direct-prometheus-apply.log" <<'YAML'
apiVersion: v1
kind: ServiceAccount
metadata: {name: direct-prometheus, namespace: inv012}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: {name: inv012-direct-prometheus}
rules:
- apiGroups: [""]
  resources: [pods, nodes, services, endpoints]
  verbs: [get, list, watch]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata: {name: inv012-direct-prometheus}
roleRef: {apiGroup: rbac.authorization.k8s.io, kind: ClusterRole, name: inv012-direct-prometheus}
subjects: [{kind: ServiceAccount, name: direct-prometheus, namespace: inv012}]
---
apiVersion: v1
kind: ConfigMap
metadata: {name: direct-prometheus, namespace: inv012}
data:
  prometheus.yml: |
    global: {scrape_interval: 1s, scrape_timeout: 500ms}
    scrape_configs:
    - job_name: annotated-pods
      kubernetes_sd_configs: [{role: pod}]
      relabel_configs:
      - {source_labels: [__meta_kubernetes_namespace], regex: inv012, action: keep}
      - {source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape], regex: "true", action: keep}
      - {source_labels: [__meta_kubernetes_pod_ip], target_label: __address__, replacement: "${1}:19112"}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: direct-prometheus, namespace: inv012}
spec:
  replicas: 1
  selector: {matchLabels: {app: direct-prometheus}}
  template:
    metadata: {labels: {app: direct-prometheus}}
    spec:
      serviceAccountName: direct-prometheus
      containers:
      - name: prometheus
        image: prom/prometheus:v3.5.0
        args: ["--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.path=/tmp/prometheus"]
        volumeMounts: [{name: config, mountPath: /etc/prometheus}]
      volumes: [{name: config, configMap: {name: direct-prometheus}}]
YAML
k -n inv012 rollout status deploy/direct-prometheus --timeout=180s >"$RESULTS_DIR/direct-prometheus-ready.log"

apply_job(){
 local name="$1" label="$2" required="$3" wait="$4" ready="$5" ttl="$6" deadline="$7" annotation="$8"
 k apply -f - >/dev/null <<YAML
apiVersion: batch/v1
kind: Job
metadata: {name: ${name}, namespace: inv012}
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: ${ttl}
  activeDeadlineSeconds: ${deadline}
  template:
    metadata:
      labels: {app: inv012, scenario: ${label}}
      annotations: {prometheus.io/scrape: "${annotation}"}
    spec:
      restartPolicy: Never
      containers:
      - name: metricshell
        image: ${IMAGE}
        imagePullPolicy: Never
        args: ["--wait=${wait}", "--required-scrapes=${required}", "--ready=${ready}"]
        ports: [{name: metrics, containerPort: 19112}]
        readinessProbe: {httpGet: {path: /readyz, port: metrics}, periodSeconds: 1, failureThreshold: 1}
YAML
}
job_logs(){ k -n inv012 logs "job/$1"; }

started="$(now_ms)"; apply_job direct-job direct 1 30s true 120 60 true; k -n inv012 wait --for=condition=complete job/direct-job --timeout=60s >/dev/null; finished="$(now_ms)"; job_logs direct-job >"$RESULTS_DIR/direct-job.log"
assert_eq direct_pod_discovery scrape_completed true "$(grep -q 'final reason=required_scrapes final_scrapes=1' "$RESULTS_DIR/direct-job.log"&&echo true||echo false)"; observe direct_pod_discovery completion_ms "$(awk -v a="$started" -v b="$finished" 'BEGIN{printf "%.3f",b-a}')" ms

# Actual ServiceMonitor generated and reconciled by Prometheus Operator.
k -n inv012 delete deploy direct-prometheus --wait=true >/dev/null
k apply -f - >/dev/null <<'YAML'
apiVersion: v1
kind: Service
metadata: {name: service-ready, namespace: inv012, labels: {monitor: service-ready}}
spec: {selector: {scenario: service-ready}, ports: [{name: metrics, port: 19112, targetPort: 19112}]}
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata: {name: service-ready, namespace: inv012}
spec:
  selector: {matchLabels: {monitor: service-ready}}
  namespaceSelector: {matchNames: [inv012]}
  endpoints: [{port: metrics, path: /metrics, interval: 1s, scrapeTimeout: 500ms}]
YAML
sleep 15
started="$(now_ms)"; apply_job service-ready service-ready 0 60s true 180 100 false; k -n inv012 wait --for=condition=complete job/service-ready --timeout=100s >/dev/null; finished="$(now_ms)"; query_time="$(($(date +%s) - 10))"; job_logs service-ready >"$RESULTS_DIR/service-ready.log"
count="$(sed -n 's/.*final_scrapes=\([0-9]*\).*/\1/p' "$RESULTS_DIR/service-ready.log"|tail -1)"; observe service_monitor http_scrapes "${count:-0}" count "aggregate handler observation; not replica attribution"; assert_eq service_monitor aggregate_http_scrapes_at_least_two true "$([[ "${count:-0}" -ge 2 ]]&&echo true||echo false)"; assert_eq readiness_true bounded_window_reason timeout "$(sed -n 's/.*final reason=\([^ ]*\).*/\1/p' "$RESULTS_DIR/service-ready.log")"; observe service_monitor completion_ms "$(awk -v a="$started" -v b="$finished" 'BEGIN{printf "%.3f",b-a}')" ms

# Query each Prometheus replica independently after the Job has completed. This proves storage in both TSDBs.
sleep 2
prometheus_pods=()
while IFS= read -r prometheus_pod; do prometheus_pods+=("$prometheus_pod"); done < <(k -n monitoring get pods -l app.kubernetes.io/name=prometheus --field-selector=status.phase=Running -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | sort)
for replica in 0 1; do
  prometheus_pod="${prometheus_pods[$replica]:-}"
  port="$((19090 + replica))"
  pf_log="$RESULTS_DIR/prometheus-replica-${replica}-port-forward.log"
  response="$RESULTS_DIR/prometheus-replica-${replica}-query.json"
  k -n monitoring port-forward "pod/$prometheus_pod" "${port}:9090" >"$pf_log" 2>&1 & pf_pid=$!
  ready=false
  for _ in $(seq 1 30); do if curl -fsS "http://127.0.0.1:${port}/-/ready" >/dev/null 2>&1; then ready=true; break; fi; sleep 1; done
  if [[ "$ready" == true ]]; then
    curl -fsS --get --data-urlencode "query=inv012_final_snapshot" --data-urlencode "time=${query_time}" "http://127.0.0.1:${port}/api/v1/query" >"$response"
  else
    printf '{"status":"error","data":{"result":[]}}\n' >"$response"
  fi
  kill "$pf_pid" >/dev/null 2>&1 || true; wait "$pf_pid" 2>/dev/null || true
  query_status="$(perl -MJSON::PP -0777 -e '$j=decode_json(<>); print $j->{status}//""' "$response")"
  series_present="$(perl -MJSON::PP -0777 -e '$j=decode_json(<>); print scalar(@{$j->{data}{result}//[]}) > 0 ? "true" : "false"' "$response")"
  sample_value="$(perl -MJSON::PP -0777 -e '$j=decode_json(<>); print $j->{data}{result}[0]{value}[1]//""' "$response")"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$replica" "$prometheus_pod" "$query_time" "$query_status" "$series_present" "$sample_value" >>"$REPLICA_EVIDENCE"
  assert_eq prometheus_replicas "prometheus_replica_${replica}_query_status" success "$query_status"
  assert_eq prometheus_replicas "prometheus_replica_${replica}_stored_sample" true "$series_present"
  assert_eq prometheus_replicas "prometheus_replica_${replica}_sample_value" 42 "$sample_value"
done

# Unready Pods are absent from Service Endpoints, so ServiceMonitor cannot complete the final scrape.
k apply -f - >/dev/null <<'YAML'
apiVersion: v1
kind: Service
metadata: {name: service-unready, namespace: inv012, labels: {monitor: service-unready}}
spec: {selector: {scenario: service-unready}, ports: [{name: metrics, port: 19112, targetPort: 19112}]}
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata: {name: service-unready, namespace: inv012}
spec:
  selector: {matchLabels: {monitor: service-unready}}
  endpoints: [{port: metrics, interval: 1s, scrapeTimeout: 500ms}]
YAML
sleep 15
apply_job service-unready service-unready 0 8s false 120 30 false; k -n inv012 wait --for=condition=complete job/service-unready --timeout=40s >/dev/null; job_logs service-unready >"$RESULTS_DIR/service-unready.log"
unready_service_scrapes="$(sed -n 's/.*final_scrapes=\([0-9]*\).*/\1/p' "$RESULTS_DIR/service-unready.log"|tail -1)"; observe readiness_false service_monitor_scrapes "${unready_service_scrapes:-0}" count "observed behavior; required-scrapes=0 forces full window"; assert_eq readiness_false bounded_window_reason timeout "$(sed -n 's/.*final reason=\([^ ]*\).*/\1/p' "$RESULTS_DIR/service-unready.log")"

# PodMonitor discovers Pod IPs directly and can scrape an unready post-workload Pod.
k apply -f - >/dev/null <<'YAML'
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata: {name: pod-unready, namespace: inv012}
spec:
  selector: {matchLabels: {scenario: pod-unready}}
  podMetricsEndpoints: [{port: metrics, path: /metrics, interval: 1s, scrapeTimeout: 500ms}]
YAML
sleep 15
apply_job pod-unready pod-unready 2 40s false 120 60 false; k -n inv012 wait --for=condition=complete job/pod-unready --timeout=80s >/dev/null; job_logs pod-unready >"$RESULTS_DIR/pod-unready.log"; count="$(sed -n 's/.*final_scrapes=\([0-9]*\).*/\1/p' "$RESULTS_DIR/pod-unready.log"|tail -1)"
assert_eq pod_monitor unready_pod_scraped true "$([[ "$count" -ge 2 ]]&&echo true||echo false)"

# activeDeadlineSeconds terminates a longer post-exit wait.
apply_job deadline deadline 0 30s true 120 5 false; set +e; k -n inv012 wait --for=condition=failed job/deadline --timeout=30s >/dev/null; deadline_wait=$?; set -e
reason="$(k -n inv012 get job deadline -o jsonpath='{.status.conditions[?(@.type=="Failed")].reason}')"; assert_eq active_deadline wait_observed 0 "$deadline_wait"; assert_eq active_deadline reason DeadlineExceeded "$reason"

# TTL removes a completed Job object after evidence is collected.
apply_job ttl ttl 0 2s true 5 30 false; k -n inv012 wait --for=condition=complete job/ttl --timeout=30s >/dev/null; job_logs ttl >"$RESULTS_DIR/ttl.log"; deleted=false
for _ in $(seq 1 30); do if ! k -n inv012 get job ttl >/dev/null 2>&1; then deleted=true; break; fi; sleep 1; done; assert_eq ttl_after_finished job_deleted true "$deleted"

# Explicit termination uses one naked Pod so no Job controller replacement obscures the result.
k -n inv012 run termination --image="$IMAGE" --image-pull-policy=Never --restart=Never -- --wait=60s --required-scrapes=0 >/dev/null; k -n inv012 wait --for=condition=Ready pod/termination --timeout=30s >/dev/null; started="$(now_ms)"; k -n inv012 delete pod termination --grace-period=1 --wait=true >/dev/null; finished="$(now_ms)"; assert_eq termination pod_removed true "$(! k -n inv012 get pod termination >/dev/null 2>&1&&echo true||echo false)"; observe termination deletion_ms "$(awk -v a="$started" -v b="$finished" 'BEGIN{printf "%.3f",b-a}')" ms

# Real CronJob scheduler: keep one 90s execution alive across the next minute and verify Forbid prevents overlap.
k apply -f - >/dev/null <<YAML
apiVersion: batch/v1
kind: CronJob
metadata: {name: overlap, namespace: inv012}
spec:
  schedule: "* * * * *"
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 1
  failedJobsHistoryLimit: 1
  jobTemplate:
    metadata: {labels: {scenario: cron-overlap}}
    spec:
      backoffLimit: 0
      template:
        metadata: {labels: {scenario: cron-overlap}}
        spec:
          restartPolicy: Never
          containers:
          - {name: metricshell, image: ${IMAGE}, imagePullPolicy: Never, args: ["--wait=180s", "--required-scrapes=0"]}
YAML
first=""
for _ in $(seq 1 90); do first="$(k -n inv012 get jobs -l scenario=cron-overlap -o name 2>/dev/null|head -1)"; [[ -n "$first" ]]&&break; sleep 1; done
assert_eq cronjob first_schedule_created true "$([[ -n "$first" ]]&&echo true||echo false)"
sleep 70
active_pods="$(k -n inv012 get pods -l scenario=cron-overlap --field-selector=status.phase=Running -o name 2>/dev/null|wc -l|tr -d ' '||true)"; jobs_count="$(k -n inv012 get jobs -l scenario=cron-overlap -o name 2>/dev/null|wc -l|tr -d ' '||true)"; policy="$(k -n inv012 get cronjob overlap -o jsonpath='{.spec.concurrencyPolicy}')"
assert_eq cronjob concurrency_policy Forbid "$policy"; assert_eq overlap one_active_pod 1 "${active_pods:-0}"; assert_eq overlap no_second_job 1 "${jobs_count:-0}"
k -n inv012 delete cronjob overlap --cascade=foreground >/dev/null

for c in toolchain two_prometheus direct_pod_discovery service_monitor prometheus_replicas readiness_true readiness_false pod_monitor active_deadline ttl_after_finished termination cronjob overlap job_post_exit; do finish_case "$c"; done
k get servicemonitors,podmonitors -A -o yaml >"$RESULTS_DIR/monitors.yaml"; k get events -A --sort-by=.lastTimestamp >"$RESULTS_DIR/events.txt"; helm get manifest inv012-monitoring --kube-context "$CTX" -n monitoring >"$RESULTS_DIR/helm-manifest.yaml"
{
 printf 'key\tvalue\n'; printf 'investigation\tINV-012\n'; printf 'runtime_under_test\tMinikube\n'; printf 'minikube_profile\t%s\n' "$PROFILE"; printf 'minikube_version\t%s\n' "$(mk version --short)"; printf 'kubernetes_version\t%s\n' "$(k version -o json|sed -n 's/.*"gitVersion": "\([^"]*\)".*/\1/p'|tail -1)"; printf 'prometheus_chart_version\t%s\n' "$CHART_VERSION"; printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"; printf 'repository_head_sha\t%s\n' "$(git -C "$REPO_DIR" rev-parse HEAD)"; printf 'benchmark_scope_diff_clean\t%s\n' "$(git -C "$REPO_DIR" diff --quiet -- research/INV-012/prototype research/INV-012/run-bench.sh&&git -C "$REPO_DIR" diff --cached --quiet -- research/INV-012/prototype research/INV-012/run-bench.sh&&echo true||echo false)"; printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"; printf 'host_uname\t%s\n' "$(uname -a|tr '\t\n' ' ')"; printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"; printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')";
} >"$RESULTS_DIR/environment.tsv"; failed="$(awk -F '\t' 'NR>1&&$5!="pass"{n++}END{print n+0}' "$ASSERTIONS")"; CURRENT_PHASE=final_assertions; printf 'status\tcomplete\nfailed_assertions\t%s\nresults_dir\t%s\n' "$failed" "$RESULTS_REF" >"$RESULTS_DIR/run-summary.tsv"; [[ "$failed" == 0 ]]
