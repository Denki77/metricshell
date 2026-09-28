PROJECT_ROOT := ..
DOCKERFILE := $(CURDIR)/Dockerfile
REVISION ?= $(shell git -C $(PROJECT_ROOT) rev-parse --verify HEAD 2>/dev/null || printf unknown)
VERSION ?= $(shell sed -n '1p' $(PROJECT_ROOT)/VERSION)
IMAGE ?= metricshell
PLATFORM ?= linux/amd64
GO_IMAGE ?= golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556

.PHONY: all fmt test integration managed-e2e managed-validation fault benchmark supply-chain supply-chain-ci ci wave1 wave2 wave3 wave4 wave5 wave6 wave7 build release

all: ci

fmt:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go fmt ./...

test:
	docker build --target test --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE)-test:local "$(PROJECT_ROOT)"

integration:
	docker build --target integration --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE)-integration:local "$(PROJECT_ROOT)"
	docker build --target php54-client -f "$(DOCKERFILE)" -t $(IMAGE)-php54-client:local "$(PROJECT_ROOT)"
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
	@volume="metricshell-php54-$$$$"; server=""; fixture=""; \
		cleanup() { test -z "$$server" || docker rm -f "$$server" >/dev/null 2>&1 || true; test -z "$$fixture" || docker rm -f "$$fixture" >/dev/null 2>&1 || true; docker volume rm "$$volume" >/dev/null 2>&1 || true; }; \
		trap cleanup EXIT INT TERM; docker volume create "$$volume" >/dev/null; \
		server="$$(docker run -d -v "$$volume:/run/shared" -e FIXTURE_SLEEP=30s $(IMAGE)-integration:local \
			--mode=managed-registry --managed-socket-path=/run/shared/private/managed.sock -- /workload-fixture)"; \
		attempt=0; until docker run --rm -v "$$volume:/run/shared" $(IMAGE)-php54-client:local \
			/run/shared/private/managed.sock 2 declare jobs counter "Jobs processed." worker >/dev/null 2>&1; do \
			attempt=$$((attempt + 1)); if test "$$attempt" -eq 30; then exit 1; fi; sleep 0.1; done; \
		docker run --rm -v "$$volume:/run/shared" $(IMAGE)-php54-client:local /run/shared/private/managed.sock 2 counter-add jobs 1.25 "worker=a=b c" >/dev/null; \
		code=0; docker run --rm -v "$$volume:/run/shared" $(IMAGE)-php54-client:local /run/shared/private/managed.sock 2 counter-add missing 1 worker=a >/dev/null || code=$$?; test "$$code" -eq 3; \
		code=0; docker run --rm -v "$$volume:/run/shared" $(IMAGE)-php54-client:local /run/shared/private/managed.sock 2 counter-add jobs not-a-number worker=a >/dev/null 2>&1 || code=$$?; test "$$code" -eq 2; \
		code=0; docker run --rm -v "$$volume:/run/shared" $(IMAGE)-php54-client:local /run/shared/private/missing.sock 1 counter-add jobs 1 >/dev/null || code=$$?; test "$$code" -eq 6; \
		docker rm -f "$$server" >/dev/null; server=""; \
		for mode in protocol unknown; do \
			fixture="$$(docker run -d -v "$$volume:/run/shared" --entrypoint /managed-client-fixture $(IMAGE)-integration:local /run/shared/private/client.sock "$$mode")"; \
			attempt=0; until docker run --rm -v "$$volume:/run/shared" --entrypoint php $(IMAGE)-php54-client:local -r 'exit(file_exists("/run/shared/private/client.sock") ? 0 : 1);'; do attempt=$$((attempt + 1)); if test "$$attempt" -eq 30; then exit 1; fi; sleep 0.1; done; \
			code=0; docker run --rm -v "$$volume:/run/shared" $(IMAGE)-php54-client:local /run/shared/private/client.sock 2 counter-add jobs 1 >/dev/null || code=$$?; \
			if test "$$mode" = protocol; then test "$$code" -eq 5; else test "$$code" -eq 7; fi; \
			docker rm -f "$$fixture" >/dev/null 2>&1 || true; fixture=""; \
		done

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

managed-e2e:
	docker build --target integration --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE)-managed-e2e:local "$(PROJECT_ROOT)"
	@network="metricshell-managed-e2e-$$$$"; volume="metricshell-managed-e2e-$$$$"; server=""; \
		cleanup() { test -z "$$server" || docker rm -f "$$server" >/dev/null 2>&1 || true; docker network rm "$$network" >/dev/null 2>&1 || true; docker volume rm "$$volume" >/dev/null 2>&1 || true; }; \
		trap cleanup EXIT INT TERM; docker network create "$$network" >/dev/null; docker volume create "$$volume" >/dev/null; \
		server="$$(docker run -d --network "$$network" --network-alias metricshell -v "$$volume:/run/shared" \
			-e METRICSHELL_MODE=managed-registry -e METRICSHELL_MANAGED_SOCKET_PATH=/run/shared/private/managed.sock \
			-e METRICSHELL_MANAGED_MAX_CONNECTIONS=64 -e METRICSHELL_MANAGED_QUEUE_CAPACITY=64 \
			-e METRICSHELL_MANAGED_PUBLICATION_INTERVAL=10ms \
			-e MANAGED_E2E_EXIT=17 \
			-e METRICSHELL_FINAL_WAIT_MODE=scrapes -e METRICSHELL_FINAL_WAIT_TIMEOUT=10s -e METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES=2 \
			$(IMAGE)-managed-e2e:local -- /managed-e2e-fixture)"; \
		output="$$(docker run --rm --network "$$network" -e FIXTURE_EXPECT_METRIC='jobs_total 33' --entrypoint /final-scrape-fixture $(IMAGE)-managed-e2e:local)"; \
		printf '%s\n' "$$output" | grep -F '"observed_final_metrics":true' >/dev/null; \
		code="$$(docker wait "$$server")"; logs="$$(docker logs "$$server" 2>&1)"; docker rm "$$server" >/dev/null; server=""; \
		test "$$code" -eq 17; printf '%s\n' "$$logs" | grep -F '"event":"fixture.managed_e2e"' | grep -F '"ack_loss_committed":true' >/dev/null; \
		printf '%s\n' "$$logs" | grep -F '"event":"fixture.managed_live"' | grep -F '"workload_running":true' >/dev/null; \
		printf '%s\n' "$$logs" | grep -F '"event":"managed.frozen"' >/dev/null; printf '%s\n' "$$logs" | grep -F '"event":"managed.final_installed"' | grep -F '"outcome":"accepted"' >/dev/null
	@volume="metricshell-managed-restart-$$$$"; server=""; \
		cleanup() { test -z "$$server" || docker rm -f "$$server" >/dev/null 2>&1 || true; docker volume rm "$$volume" >/dev/null 2>&1 || true; true; }; \
		trap cleanup EXIT INT TERM; docker volume create "$$volume" >/dev/null; \
		for run in 1 2; do server="$$(docker run -d -v "$$volume:/run/shared" -e MANAGED_E2E_MODE=restart \
			-e METRICSHELL_MODE=managed-registry -e METRICSHELL_MANAGED_SOCKET_PATH=/run/shared/private/managed.sock \
			$(IMAGE)-managed-e2e:local -- /managed-e2e-fixture)"; code="$$(docker wait "$$server")"; logs="$$(docker logs "$$server" 2>&1)"; docker rm "$$server" >/dev/null; server=""; \
			test "$$code" -eq 0; printf '%s\n' "$$logs" | grep -F '"event":"fixture.managed_restart"' | grep -F '"empty_epoch":true' >/dev/null; done
	@volume="metricshell-managed-partial-$$$$"; server=""; \
		cleanup() { test -z "$$server" || docker rm -f "$$server" >/dev/null 2>&1 || true; docker volume rm "$$volume" >/dev/null 2>&1 || true; }; \
		trap cleanup EXIT INT TERM; docker volume create "$$volume" >/dev/null; \
		server="$$(docker run -d -v "$$volume:/run/shared" -e MANAGED_E2E_MODE=partial-shutdown \
			-e METRICSHELL_MODE=managed-registry -e METRICSHELL_MANAGED_SOCKET_PATH=/run/shared/private/managed.sock \
			-e METRICSHELL_MANAGED_READ_TIMEOUT=1m -e METRICSHELL_SHUTDOWN_TOTAL_GRACE=1s \
			-e METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT=100ms -e METRICSHELL_SHUTDOWN_RESERVE=250ms \
			$(IMAGE)-managed-e2e:local -- /managed-e2e-fixture)"; \
		attempt=0; until docker logs "$$server" 2>&1 | grep -F '"event":"fixture.partial_ready"' >/dev/null; do attempt=$$((attempt + 1)); if test "$$attempt" -eq 50; then exit 1; fi; sleep 0.1; done; \
		started=$$(date +%s); docker kill --signal TERM "$$server" >/dev/null || exit 1; code="$$(docker wait "$$server")"; elapsed=$$(( $$(date +%s) - started )); \
		logs="$$(docker logs "$$server" 2>&1)"; docker rm "$$server" >/dev/null; server=""; test "$$code" -eq 137; test "$$elapsed" -le 3; \
		printf '%s\n' "$$logs" | grep -F '"event":"managed.final_installed"' | grep -F '"outcome":"accepted"' >/dev/null

managed-validation: integration managed-e2e
	docker build --target managed-validation-artifacts --build-arg REVISION="$(REVISION)" \
		--output type=local,dest="$(CURDIR)/dist/managed-validation" -f "$(DOCKERFILE)" "$(PROJECT_ROOT)"
	test -s "$(CURDIR)/dist/managed-validation/validation/raw-race-tests.txt"
	test -s "$(CURDIR)/dist/managed-validation/validation/raw-benchmark.txt"
	test -s "$(CURDIR)/dist/managed-validation/validation/go-environment.json"
	test -s "$(CURDIR)/dist/managed-validation/validation/metadata.json"

benchmark:
	docker build --target benchmark-artifacts --build-arg REVISION="$(REVISION)" --output type=local,dest="$(CURDIR)/dist/benchmark" -f "$(DOCKERFILE)" "$(PROJECT_ROOT)"

supply-chain:
	@test -n "$(RELEASE_SIGNING_KEY_FILE)"
	@test -n "$(RELEASE_SIGNING_PUBLIC_KEY_FILE)"
	docker build --target supply-chain-artifacts --build-arg REVISION="$(REVISION)" \
		--secret id=release_signing_key,src="$(RELEASE_SIGNING_KEY_FILE)" \
		--secret id=release_public_key,src="$(RELEASE_SIGNING_PUBLIC_KEY_FILE)" \
		--output type=local,dest="$(CURDIR)/dist/supply-chain" -f "$(DOCKERFILE)" "$(PROJECT_ROOT)"

supply-chain-ci:
	@temp="$$(mktemp -d)"; \
		cleanup() { rm -rf "$$temp"; }; \
		trap cleanup EXIT INT TERM; \
		docker run --rm -v "$(CURDIR):/src" -v "$$temp:/keys" -w /src $(GO_IMAGE) \
			go run ./internal/testfixture/signingkey /keys/private.hex /keys/public.hex; \
		$(MAKE) supply-chain IMAGE="$(IMAGE)" REVISION="$(REVISION)" \
			RELEASE_SIGNING_KEY_FILE="$$temp/private.hex" \
			RELEASE_SIGNING_PUBLIC_KEY_FILE="$$temp/public.hex"

ci: test integration fault benchmark supply-chain-ci

wave1: ci

wave2: ci

wave3: ci

wave4: ci

wave5: ci

wave6: test integration managed-e2e fault benchmark supply-chain-ci

wave7: test managed-validation fault benchmark supply-chain-ci

build:
	docker build --target runtime --platform "$(PLATFORM)" --build-arg VERSION="$(VERSION)" --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE):local "$(PROJECT_ROOT)"

release:
	docker build --target release --build-arg REVISION="$(REVISION)" --output type=local,dest="$(CURDIR)/dist" -f "$(DOCKERFILE)" "$(PROJECT_ROOT)"
