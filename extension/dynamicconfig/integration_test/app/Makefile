.PHONY: do-unit-tests-with-cover
do-unit-tests-with-cover:
	@echo "no units tests in $(shell pwd)"

.PHONY: lint
lint:
	@echo "skipping lint for dynamic config integration tests"

%: force
	@$(MAKE) -f ../../../../Makefile.Common $@

force: ;
