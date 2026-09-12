PYTHON ?= $(if $(wildcard .venv/bin/python),./.venv/bin/python,python3)
NODE ?= node

.PHONY: install-dev test test-v2 check-python check-frontend check-frontend-v2 check database-readiness

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

check-frontend:
	$(NODE) --check frontend/app.js

check-frontend-v2:
	cd frontend/v2 && npm ci --ignore-scripts && npm run build

check: check-python check-frontend check-frontend-v2 test
