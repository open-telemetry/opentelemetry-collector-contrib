include ../Makefile.Common

.PHONY: list-tests
list-tests:
	TESTBED_CONFIG=local.yaml $(GOTEST) -v ./tests --test.list '.*' | sed '$$d'

.PHONY: run-tests
run-tests:
	./runtests.sh

.PHONY: list-stability-tests
list-stability-tests:
	TESTBED_CONFIG=local.yaml $(GOTEST) -v ./stabilitytests --test.list '.*' | sed '$$d'

.PHONY: run-stability-tests
run-stability-tests:
	TESTCASE_DURATION=1h TEST_ARGS="$${TEST_ARGS} -timeout 70m" TESTS_DIR=stabilitytests ./runtests.sh

.PHONY: install-tools
install-tools:
	go install github.com/jstemmer/go-junit-report
