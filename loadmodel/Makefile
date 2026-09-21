# P7 load harness Makefile

ROOT := $(shell pwd)
GO   := go
K6   := /opt/homebrew/bin/k6

.PHONY: build bench test clean all smoke cached voice writes cold outage burst

all: build bench test

build:
	cd $(ROOT) && $(GO) build -o bin/loadtestd ./cmd/loadtestd
	cd $(ROOT) && $(GO) build -o bin/dummyd ./cmd/dummyd
	cd $(ROOT) && $(GO) build -o bin/voiceloadd ./cmd/voiceloadd

bench:
	cd $(ROOT) && $(GO) test -bench=. -benchmem -benchtime=2s -run=^$ ./loadfixtures/

test:
	cd $(ROOT) && $(GO) test -count=1 -race ./...

clean:
	rm -rf bin/
	rm -rf reports/

smoke: build
	bash scripts/run_scenario.sh smoke

cached: build
	bash scripts/run_scenario.sh cached_1k

voice: build
	bash scripts/run_scenario.sh voice_20

writes: build
	bash scripts/run_scenario.sh writes_50

cold: build
	bash scripts/run_scenario.sh cold_cache

outage: build
	bash scripts/run_scenario.sh source_outage

burst: build
	bash scripts/run_scenario.sh burst
