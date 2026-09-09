PROJECT_ROOT := ..
DOCKERFILE := $(CURDIR)/Dockerfile
REVISION ?= $(shell git -C $(PROJECT_ROOT) rev-parse --verify HEAD 2>/dev/null || printf unknown)
VERSION ?= $(shell sed -n '1p' $(PROJECT_ROOT)/VERSION)
IMAGE ?= metricshell
PLATFORM ?= linux/amd64
GO_IMAGE ?= golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556

.PHONY: all fmt test integration fault ci wave1 wave2 wave3 wave4 wave5 wave6 build release

all: ci

fmt:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go fmt ./...

test:
	docker build --target test --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE)-test:local "$(PROJECT_ROOT)"

integration:
	docker build --target integration --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE)-integration:local "$(PROJECT_ROOT)"
	@code=0; output="$$(docker run --rm $(IMAGE)-integration:local -- /workload-fixture "space value" "" "Привет" --version --)" || code=$$?; \
    	test "$$code" -eq 0; \
		printf '%s\n' "$$output" | grep -F '"argv":["space value","","Привет","--version","--"]' >/dev/null; \
		printf '%s\n' "$$output" | grep -F '"ppid":1' >/dev/null
	@code=0; docker run --rm -e FIXTURE_EXIT=17 $(IMAGE)-integration:local -- /workload-fixture >/dev/null || code=$$?; test "$$code" -eq 17
	@code=0; docker run --rm -e FIXTURE_SIGNAL=15 $(IMAGE)-integration:local -- /workload-fixture >/dev/null || code=$$?; test "$$code" -eq 143
	@code=0; output="$$(docker run --rm $(IMAGE)-integration:local -- /missing-workload 2>&1)" || code=$$?; \
		test "$$code" -eq 73; printf '%s\n' "$$output" | grep -F '"event":"workload.start_failed"' >/dev/null; \
		printf '%s\n' "$$output" | grep -F '"error_code":"WORKLOAD_START_FAILED"' >/dev/null
	@container="$$(docker run -d -e FIXTURE_SLEEP=10s $(IMAGE)-integration:local -- /workload-fixture)"; \
		docker kill --signal TERM "$$container" >/dev/null; code="$$(docker wait "$$container")"; docker rm "$$container" >/dev/null; \
		test "$$code" -ne 0; test "$$code" -ne 73
	@code=0; output="$$(docker run --rm $(IMAGE)-integration:local -- /process-group-fixture 2>&1)" || code=$$?; \
		test "$$code" -eq 0; \
		printf '%s\n' "$$output" | grep -F '"event":"workload.started"' >/dev/null; \
		printf '%s\n' "$$output" | grep -F '"workload_pgid":' >/dev/null
	@attempt=0; while test "$$attempt" -lt 25; do \
		docker run --rm $(IMAGE)-integration:local -- /workload-fixture >/dev/null 2>&1 || exit 1; \
		attempt=$$((attempt + 1)); \
	done
	@code=0; output="$$(docker run --rm -e FIXTURE_SIGNALS=HUP,QUIT,HUP,QUIT,HUP $(IMAGE)-integration:local -- /signal-fixture 2>&1)" || code=$$?; \
		test "$$code" -eq 0; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"workload.signal_forwarded"')" -eq 5; \
		test "$$(printf '%s\n' "$$output" | grep '"event":"workload.signal_forwarded"' | sed -n 's/.*"signal":"\([^"]*\)".*/\1/p' | paste -sd, -)" = "HUP,QUIT,HUP,QUIT,HUP"; \
		printf '%s\n' "$$output" | grep '"state":"running".*"signal":"HUP"' >/dev/null; \
		printf '%s\n' "$$output" | grep -F '"signal":"HUP","count":5' >/dev/null; \
		printf '%s\n' "$$output" | grep -F '"signal":"HUP"' >/dev/null; \
		printf '%s\n' "$$output" | grep -F '"signal":"QUIT"' >/dev/null
	@attempt=0; while test "$$attempt" -lt 25; do \
		code=0; output="$$(docker run --rm -e FIXTURE_SIGNALS=TERM -e FIXTURE_EXIT_DURING_FORWARD=1 $(IMAGE)-integration:local -- /signal-fixture 2>&1)" || code=$$?; \
		test "$$code" -eq 0; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"workload.signal_ignored"')" -eq 1; \
		printf '%s\n' "$$output" | grep '"event":"workload.signal_ignored".*"reason":"target_exited"' >/dev/null; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"workload.signal_forwarded"')" -eq 0; \
		attempt=$$((attempt + 1)); \
	done
	@for requested in TERM INT; do \
		container="$$(docker run -d -e FIXTURE_PASSIVE_SIGNAL="$$requested" $(IMAGE)-integration:local -- /signal-fixture)"; \
		attempt=0; until docker logs "$$container" 2>&1 | grep -F '"event":"fixture.ready"' >/dev/null; do \
			attempt=$$((attempt + 1)); \
			if test "$$attempt" -eq 30; then docker rm -f "$$container" >/dev/null; exit 1; fi; \
			sleep 0.1; \
		done; \
		docker kill --signal "$$requested" "$$container" >/dev/null; \
		code="$$(docker wait "$$container")"; output="$$(docker logs "$$container" 2>&1)"; docker rm "$$container" >/dev/null; \
		test "$$code" -eq 0; \
		printf '%s\n' "$$output" | grep -F '"event":"workload.signal_forwarded"' >/dev/null; \
		printf '%s\n' "$$output" | grep -F "\"signal\":\"$$requested\"" >/dev/null; \
	done
	@code=0; output="$$(docker run --rm -e REAPER_MODE=primary-before-child $(IMAGE)-integration:local -- /reaper-fixture 2>&1)" || code=$$?; \
		test "$$code" -eq 21; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"workload.exited"')" -eq 1; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"child.reaped".*"kind":"direct"')" -eq 1; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"child.reaped".*"kind":"adopted"')" -eq 1; \
		test "$$(printf '%s\n' "$$output" | grep -n -m1 '"event":"workload.exited"' | cut -d: -f1)" -lt "$$(printf '%s\n' "$$output" | grep -n -m1 '"event":"child.reaped".*"kind":"adopted"' | cut -d: -f1)"
	@code=0; output="$$(docker run --init --rm -e REAPER_MODE=child-before-primary $(IMAGE)-integration:local -- /reaper-fixture 2>&1)" || code=$$?; \
		test "$$code" -eq 22; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"workload.exited"')" -eq 1; \
		test "$$(printf '%s\n' "$$output" | grep -n -m1 '"event":"child.reaped".*"kind":"adopted"' | cut -d: -f1)" -lt "$$(printf '%s\n' "$$output" | grep -n -m1 '"event":"workload.exited"' | cut -d: -f1)"
	@code=0; output="$$(docker run --rm -e REAPER_MODE=burst $(IMAGE)-integration:local -- /reaper-fixture 2>&1)" || code=$$?; \
		test "$$code" -eq 23; \
		printf '%s\n' "$$output" | grep -F '"event":"fixture.audit","zombies":0' >/dev/null; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"workload.exited"')" -eq 1; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"child.reaped".*"kind":"direct"')" -eq 1; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"child.reaped".*"kind":"adopted"')" -eq 64
	@code=0; output="$$(docker run --rm --entrypoint /result-matrix-fixture $(IMAGE)-integration:local 2>&1)" || code=$$?; \
		test "$$code" -eq 0; \
		printf '%s\n' "$$output" | grep -F '"event":"fixture.result_matrix","configurations_checked":1,"exits_checked":256,"signals_checked":2' >/dev/null
	@for mode in cooperative ignore fork-after-signal; do \
		container="$$(docker run -d -e TERMINATION_MODE="$$mode" -e METRICSHELL_SHUTDOWN_TOTAL_GRACE=1s -e METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT=100ms -e METRICSHELL_SHUTDOWN_RESERVE=250ms $(IMAGE)-integration:local -- /termination-fixture)"; \
		attempt=0; until docker logs "$$container" 2>&1 | grep -F '"event":"fixture.ready"' >/dev/null; do \
			attempt=$$((attempt + 1)); \
			if test "$$attempt" -eq 30; then docker rm -f "$$container" >/dev/null; exit 1; fi; \
			sleep 0.1; \
		done; \
		docker kill --signal TERM "$$container" >/dev/null; \
		code="$$(docker wait "$$container")"; output="$$(docker logs "$$container" 2>&1)"; docker rm "$$container" >/dev/null; \
		if test "$$mode" = cooperative; then test "$$code" -eq 0; test "$$(printf '%s\n' "$$output" | grep -c '"event":"shutdown.forced"')" -eq 0; \
		else test "$$code" -eq 137; test "$$(printf '%s\n' "$$output" | grep -c '"event":"shutdown.forced"')" -eq 1; fi; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"shutdown.started"')" -eq 1; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"shutdown.completed"')" -eq 1; \
		if test "$$mode" = fork-after-signal; then printf '%s\n' "$$output" | grep '"event":"child.reaped".*"kind":"adopted"' >/dev/null; fi; \
	done
	@container="$$(docker run -d -e TERMINATION_MODE=ignore -e METRICSHELL_SHUTDOWN_TOTAL_GRACE=1s -e METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT=750ms -e METRICSHELL_SHUTDOWN_RESERVE=250ms $(IMAGE)-integration:local -- /termination-fixture)"; \
		attempt=0; until docker logs "$$container" 2>&1 | grep -F '"event":"fixture.ready"' >/dev/null; do attempt=$$((attempt + 1)); if test "$$attempt" -eq 30; then docker rm -f "$$container" >/dev/null; exit 1; fi; sleep 0.1; done; \
		docker kill --signal TERM "$$container" >/dev/null; docker kill --signal TERM "$$container" >/dev/null; \
		code="$$(docker wait "$$container")"; output="$$(docker logs "$$container" 2>&1)"; docker rm "$$container" >/dev/null; \
		test "$$code" -eq 137; test "$$(printf '%s\n' "$$output" | grep -c '"event":"shutdown.forced"')" -eq 1
	@container="$$(docker run -d -e TERMINATION_MODE=ignore -e METRICSHELL_SHUTDOWN_TOTAL_GRACE=1s -e METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT=0 -e METRICSHELL_SHUTDOWN_RESERVE=1s $(IMAGE)-integration:local -- /termination-fixture)"; \
		attempt=0; until docker logs "$$container" 2>&1 | grep -F '"event":"fixture.ready"' >/dev/null; do attempt=$$((attempt + 1)); if test "$$attempt" -eq 30; then docker rm -f "$$container" >/dev/null; exit 1; fi; sleep 0.1; done; \
		docker kill --signal TERM "$$container" >/dev/null; code="$$(docker wait "$$container")"; output="$$(docker logs "$$container" 2>&1)"; docker rm "$$container" >/dev/null; \
		test "$$code" -eq 137; printf '%s\n' "$$output" | grep -F '"event":"shutdown.started"' | grep -F '"remaining_ms":1000' >/dev/null
	@output="$$(docker run --rm --entrypoint /probe-fixture $(IMAGE)-integration:local)"; \
		printf '%s\n' "$$output" | grep -F '"event":"fixture.probes"' | grep -F '"requests_checked":18' | grep -F '"states_checked":8' >/dev/null
	@network="metricshell-wave5-$$$$"; server=""; \
		cleanup() { if test -n "$$server"; then docker rm -f "$$server" >/dev/null 2>&1 || true; fi; docker network rm "$$network" >/dev/null 2>&1 || true; }; \
		trap cleanup EXIT INT TERM; \
		docker network create "$$network" >/dev/null; \
		server="$$(docker run -d --network "$$network" --network-alias metricshell \
			-e METRICSHELL_FINAL_WAIT_MODE=scrapes -e METRICSHELL_FINAL_WAIT_TIMEOUT=5s \
			-e METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES=2 $(IMAGE)-integration:local -- /workload-fixture)"; \
		output="$$(docker run --rm --network "$$network" --entrypoint /final-scrape-fixture $(IMAGE)-integration:local)"; \
		printf '%s\n' "$$output" | grep -F '"event":"fixture.final_scrape"' | grep -F '"observed_final_metrics":true' >/dev/null; \
		code="$$(docker wait "$$server")"; logs="$$(docker logs "$$server" 2>&1)"; docker rm "$$server" >/dev/null; server=""; \
		test "$$code" -eq 0; \
		printf '%s\n' "$$logs" | grep '"event":"final_wait.started".*"mode":"scrapes"' >/dev/null; \
		test "$$(printf '%s\n' "$$logs" | grep -c '"event":"final_scrape.counted"')" -eq 2; \
		printf '%s\n' "$$logs" | grep '"event":"final_wait.completed".*"reason":"required_scrapes"' >/dev/null

fault:
	docker build --target integration --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE)-fault:local "$(PROJECT_ROOT)"
	@for mode in malformed-flood slow-disconnect scrape-race; do \
		output="$$(docker run --rm -e METRICSHELL_INGESTION_TRANSPORT=http -e FAULTSUITE_MODE="$$mode" $(IMAGE)-fault:local -- /faultsuite-fixture 2>&1)"; \
		printf '%s\n' "$$output" | grep -F '"event":"fixture.faultsuite"' | grep -F "\"mode\":\"$$mode\"" | grep -F '"ok":true' >/dev/null; \
	done
	@output="$$(docker run --rm -e METRICSHELL_INGESTION_TRANSPORT=http -e METRICSHELL_MAX_CONCURRENT_INGESTIONS=1 -e METRICSHELL_MAX_PENDING_INGESTIONS=0 -e FAULTSUITE_MODE=saturation $(IMAGE)-fault:local -- /faultsuite-fixture 2>&1)"; \
		printf '%s\n' "$$output" | grep -F '"event":"fixture.faultsuite","mode":"saturation","ok":true' >/dev/null; \
		printf '%s\n' "$$output" | grep -F '"event":"ingestion.overloaded"' >/dev/null
	@code=0; output="$$(docker run --rm -e METRICSHELL_INGESTION_TRANSPORT=http -e METRICSHELL_EXPOSITION_LISTEN=127.0.0.1:9091 -e METRICSHELL_HTTP_INGESTION_LISTEN=127.0.0.1:9091 $(IMAGE)-fault:local -- /faultsuite-fixture 2>&1)" || code=$$?; \
		test "$$code" -eq 72; \
		printf '%s\n' "$$output" | grep -F '"event":"endpoint.bind_failed"' >/dev/null; \
		test "$$(printf '%s\n' "$$output" | grep -c '"event":"workload.started"')" -eq 0
	@code=0; docker run --rm --memory=32m -e METRICSHELL_INGESTION_TRANSPORT=http -e FAULTSUITE_MODE=oom $(IMAGE)-fault:local -- /faultsuite-fixture >/dev/null 2>&1 || code=$$?; \
		test "$$code" -eq 137

ci: test integration fault

wave1: ci

wave2: ci

wave3: ci

wave4: ci

wave5: ci

build:
	docker build --target runtime --platform "$(PLATFORM)" --build-arg VERSION="$(VERSION)" --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE):local "$(PROJECT_ROOT)"

release:
	docker build --target release --build-arg REVISION="$(REVISION)" --output type=local,dest="$(CURDIR)/dist" -f "$(DOCKERFILE)" "$(PROJECT_ROOT)"
