.PHONY: do-unit-tests-with-cover
do-unit-tests-with-cover:
	@echo "no units tests in $(shell pwd)"

.PHONY: lint
lint:
	@echo "skipping lint for integration test sample app"

%: force
	@$(MAKE) -f ../../../../Makefile.Common $@

force: ;
