BENCH_TIME  ?= 3s
BENCH_COUNT ?= 1
BENCH_FILTER ?= .
PKG         := ./...

.PHONY: all build test bench bench-compare bench-save lint clean

all: test

build:
	go build -o cdagee ./cmd/cdagee/

test:
	go test -race $(PKG)

bench:
	go test $(PKG) -run='^$$' -bench=$(BENCH_FILTER) -benchmem \
		-benchtime=$(BENCH_TIME) -count=$(BENCH_COUNT)

bench-compare:
	@which benchstat > /dev/null || go install golang.org/x/perf/cmd/benchstat@latest
	go test $(PKG) -run='^$$' -bench=$(BENCH_FILTER) -benchmem \
		-benchtime=$(BENCH_TIME) -count=6 | tee /tmp/bench-new.txt
	@if [ -f /tmp/bench-old.txt ]; then \
		benchstat /tmp/bench-old.txt /tmp/bench-new.txt; \
	else \
		echo "No baseline found. Run 'make bench-save' first, then make changes, then re-run 'make bench-compare'."; \
	fi

bench-save:
	@which benchstat > /dev/null || go install golang.org/x/perf/cmd/benchstat@latest
	go test $(PKG) -run='^$$' -bench=$(BENCH_FILTER) -benchmem \
		-benchtime=$(BENCH_TIME) -count=6 | tee /tmp/bench-old.txt
	@echo "Baseline saved to /tmp/bench-old.txt"

lint:
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run $(PKG)

clean:
	go clean -testcache
	rm -f cdagee *.test cpu.prof mem.prof
