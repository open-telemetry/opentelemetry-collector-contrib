include ../Makefile.Common

.PHONY: list-tests
list-tests:
	RUN_TESTBED=1 $(GOTEST) -v ./tests --test.list '.*' | grep "^Test"

.PHONY: run-tests
run-tests: $(GOJUNIT)
	GOJUNIT=$(GOJUNIT) ./runtests.sh

.PHONY: list-loadtest-tests
list-loadtest-tests:
	RUN_TESTBED=1 $(GOTEST) -v ./tests --test.list '.*' | grep "^Test"

.PHONY: list-stability-tests
list-stability-tests:
	RUN_TESTBED=1 $(GOTEST) -v ./stabilitytests --test.list '.*' | grep "^Test"

.PHONY: run-stability-tests
run-stability-tests:
	TESTCASE_DURATION=1h TEST_ARGS="$${TEST_ARGS} -timeout 70m" TESTS_DIR=stabilitytests ./runtests.sh

.PHONY: list-correctness-metrics-tests
list-correctness-metrics-tests:
	RUN_TESTBED=1 $(GOTEST) -v ./correctnesstests/metrics --test.list '.*' | grep "^Test"

.PHONY: run-correctness-metrics-tests
run-correctness-metrics-tests: $(GOJUNIT)
	TESTS_DIR=correctnesstests/metrics GOJUNIT=$(GOJUNIT) ./runtests.sh

.PHONY: list-correctness-traces-tests
list-correctness-traces-tests:
	RUN_TESTBED=1 $(GOTEST) -v ./correctnesstests/traces --test.list '.*' | grep "^Test"

.PHONY: run-correctness-traces-tests
run-correctness-traces-tests: $(GOJUNIT)
	TESTS_DIR=correctnesstests/traces GOJUNIT=$(GOJUNIT) ./runtests.sh
