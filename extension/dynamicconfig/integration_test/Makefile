.PHONY: do-unit-tests-with-cover
do-unit-tests-with-cover:
	echo "no units tests in $(shell pwd)"

.PHONY: run-integration-tests-with-cover
do-integration-tests-with-cover:
	echo "run integration test manually for $(shell pwd)"

%: force
	@$(MAKE) -f ../../../Makefile.Common $@

force: ;
