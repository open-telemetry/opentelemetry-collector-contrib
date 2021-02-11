include ../Makefile.Common

.PHONY: list-tests
list-tests:
	RUN_TESTBED=1 $(GOTEST) -v ./tests --test.list '.*' | grep "^Test"

.PHONY: run-tests
run-tests:
	./runtests.sh

.PHONY: run-tests-unstable
run-tests-unstable:
	TESTS_DIR=tests_unstable_exe ./runtests.sh

.PHONY: list-stability-tests
list-stability-tests:
	RUN_TESTBED=1 $(GOTEST) -v ./stabilitytests --test.list '.*' | grep "^Test"

.PHONY: run-stability-tests
run-stability-tests:
	TESTCASE_DURATION=1h TEST_ARGS="$${TEST_ARGS} -timeout 70m" TESTS_DIR=stabilitytests ./runtests.sh

.PHONY: install-tools
install-tools:
	go install github.com/jstemmer/go-junit-report
