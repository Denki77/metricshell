PROJECT_ROOT := ..
DOCKERFILE := $(CURDIR)/Dockerfile
REVISION ?= $(shell git -C $(PROJECT_ROOT) rev-parse --verify HEAD 2>/dev/null || printf unknown)
IMAGE ?= metricshell
PLATFORM ?= linux/amd64

.PHONY: all test ci build

all: ci

test:
	docker build --target test --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE)-test:local "$(PROJECT_ROOT)"

ci: test

build:
	docker build --target runtime --platform "$(PLATFORM)" --build-arg REVISION="$(REVISION)" -f "$(DOCKERFILE)" -t $(IMAGE):local "$(PROJECT_ROOT)"
