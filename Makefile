PYTHON ?= conda run -n ddi-agent python

.PHONY: init-db run-local e2e-smoke test-go test-python

init-db:
	./scripts/init_db.sh

run-local:
	./scripts/run_local.sh

e2e-smoke:
	./scripts/e2e_smoke.sh

test-go:
	go test ./...

test-python:
	$(PYTHON) -m pytest python/runtime/tests -q
