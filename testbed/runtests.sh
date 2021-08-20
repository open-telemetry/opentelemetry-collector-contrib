#!/bin/bash

set -e

TESTS_DIR=${TESTS_DIR:-tests}

cd ${TESTS_DIR}

SED="sed"

PASS_COLOR=$(printf "\033[32mPASS\033[0m")
FAIL_COLOR=$(printf "\033[31mFAIL\033[0m")
TEST_COLORIZE="${SED} 's/PASS/${PASS_COLOR}/' | ${SED} 's/FAIL/${FAIL_COLOR}/'"

mkdir -p results/junit

RUN_TESTBED=1 go test -v ${TEST_ARGS} 2>&1 | tee results/testoutput.log | bash -c "${TEST_COLORIZE}"

testStatus=${PIPESTATUS[0]}

go-junit-report < results/testoutput.log > results/junit/results.xml

bash -c "cat results/TESTRESULTS.md | ${TEST_COLORIZE}"

exit ${testStatus}
