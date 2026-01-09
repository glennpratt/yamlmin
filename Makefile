SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c

export CGO_ENABLED := 0

FIXTURE := pkg/yamlmin/testdata/fixture.yaml

.PHONY: all
all: fmt lint-fix test benchmark

.PHONY: fmt
fmt:
	golangci-lint fmt ./...

.PHONY: fmt-check
fmt-check:
	golangci-lint fmt --diff ./...

.PHONY: lint
lint: fmt-check
	golangci-lint run

.PHONY: lint-fix
lint-fix: fmt
	golangci-lint run --fix

.PHONY: test
test: unit-test integration-test

.PHONY: unit-test
unit-test:
	go test -v ./...

.PHONY: integration-test
integration-test:
	dyff between --set-exit-code $(FIXTURE) <(go run . < $(FIXTURE))
	@echo "Integration test passed"

.PHONY: benchmark
benchmark:
	go test -bench=. -benchmem ./...
