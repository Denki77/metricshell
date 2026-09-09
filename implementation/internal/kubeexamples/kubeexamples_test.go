package kubeexamples

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJobIntegrationManifestsEncodeFinalScrapeContract(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "examples", "kubernetes", "job-integration")
	job := readExample(t, root, "job-direct-discovery.yaml")
	podMonitor := readExample(t, root, "podmonitor.yaml")
	verification := readExample(t, root, "final-sample-verification.md")

	requireAll(t, job,
		"kind: Job",
		"activeDeadlineSeconds: 95",
		"ttlSecondsAfterFinished: 300",
		"terminationGracePeriodSeconds: 32",
		"restartPolicy: Never",
		"automountServiceAccountToken: false",
		"METRICSHELL_FINAL_WAIT_MODE",
		"value: scrapes",
		"METRICSHELL_FINAL_WAIT_TIMEOUT",
		"value: 60s",
		"readinessProbe:",
		"path: /readyz",
		"path: /healthz",
		"containerPort: 9090",
		"metricshell.io/scrape: \"true\"",
		"readOnlyRootFilesystem: true",
		"runAsNonRoot: true",
		"drop: [\"ALL\"]",
		"emptyDir:",
	)
	requireNotContains(t, job, "containerPort: 9091")

	requireAll(t, podMonitor,
		"kind: PodMonitor",
		"metricshell.io/scrape: \"true\"",
		"port: metrics",
		"path: /metrics",
		"scrapeTimeout: 2s",
		"targetLabel: metricshell_replica",
	)
	requireAll(t, verification,
		"each configured Prometheus replica",
		"range query",
		"stale marker",
	)
}

func TestLifecycleControlManifestsEncodeOuterBounds(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "examples", "kubernetes", "lifecycle-controls")
	job := readExample(t, root, "job-lifecycle.yaml")
	cron := readExample(t, root, "cronjob-forbid.yaml")
	readme := readExample(t, root, "README.md")

	requireAll(t, job,
		"kind: Job",
		"activeDeadlineSeconds: 95",
		"ttlSecondsAfterFinished: 300",
		"backoffLimit: 0",
		"restartPolicy: Never",
		"terminationGracePeriodSeconds: 32",
		"METRICSHELL_SHUTDOWN_TOTAL_GRACE",
		"value: 30s",
		"METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT",
		"value: 28s",
		"METRICSHELL_SHUTDOWN_RESERVE",
		"value: 2s",
	)
	requireAll(t, cron,
		"kind: CronJob",
		"concurrencyPolicy: Forbid",
		"activeDeadlineSeconds: 95",
		"ttlSecondsAfterFinished: 300",
		"restartPolicy: Never",
		"terminationGracePeriodSeconds: 32",
	)
	requireAll(t, readme,
		"measurable two-second margin",
		"`CronJob.concurrencyPolicy: Forbid`",
	)
}

func readExample(t *testing.T, root, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func requireAll(t *testing.T, content string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in content", needle)
		}
	}
}

func requireNotContains(t *testing.T, content, needle string) {
	t.Helper()
	if strings.Contains(content, needle) {
		t.Fatalf("did not expect %q in content", needle)
	}
}
