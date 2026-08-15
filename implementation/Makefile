PROJECT_ROOT := ..
DOCKERFILE := $(CURDIR)/Dockerfile
REVISION ?= $(shell git -C $(PROJECT_ROOT) rev-parse --verify HEAD 2>/dev/null || printf unknown)
IMAGE ?= metricshell
PLATFORM ?= linux/amd64

.PHONY: all test integration ci build

all: ci

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
	@code=0; docker run --rm $(IMAGE)-integration:local >/dev/null 2>&1 || code=$$?; test "$$code" -eq 64
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

ci: test integration

build:
	docker build --target runtime --platform "$(PLATFORM)" --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE):local "$(PROJECT_ROOT)"
