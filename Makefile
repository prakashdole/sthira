PYTHON ?= $(if $(wildcard .venv/bin/python),./.venv/bin/python,python3)
NODE ?= node
GO ?= $(if $(wildcard /opt/homebrew/opt/go/bin/go),/opt/homebrew/opt/go/bin/go,go)

.PHONY: install-dev test test-v2 check-python check-frontend-v2 check database-readiness check-go test-go check-adapters

install-dev:
	$(PYTHON) -m pip install -e '.[dev]'

test:
	$(PYTHON) -m pytest -q

test-v2:
	$(PYTHON) -m pytest -q tests/test_v2_phase0.py tests/test_v2_openapi.py

database-readiness:
	$(PYTHON) scripts/check_v2_database.py

check-python:
	$(PYTHON) -m compileall -q src tests

check-adapters:
	$(PYTHON) -m pytest -q tests/test_b2_adapters.py tests/test_v2_real_adapters.py

check-frontend-v2:
	cd frontend/v2 && npm ci --ignore-scripts && npm test && npm run build

check-go:
	cd backend && test -z "$$($(GO)fmt -l .)"
	cd backend && $(GO) vet ./...
	cd backend && $(GO) build ./...

test-go:
	cd backend && $(GO) test ./...

check: check-go test-go check-frontend-v2 check-adapters check-python
