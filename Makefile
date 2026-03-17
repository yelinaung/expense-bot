.PHONY: build test test-coverage test-race test-db-up test-db-down test-integration coverage-html lint fmt clean release push

TASKS = build test test-coverage test-race test-db-up test-db-down test-integration coverage-html lint fmt clean release push

$(TASKS):
	mise run $@
