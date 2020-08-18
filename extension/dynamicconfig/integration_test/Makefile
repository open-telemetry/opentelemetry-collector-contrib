.PHONY: do-unit-tests-with-cover
do-unit-tests-with-cover:
	@echo "no units tests in $(shell pwd)"

.PHONY: run-integration-tests-with-cover
do-integration-tests-with-cover:
	@echo "run integration test manually for $(shell pwd)"

.PHONY: lint
lint:
	@echo "skipping lint for dynamic config integration tests"

%: force
	@$(MAKE) -f ../../../Makefile.Common $@

force: ;
