#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

GOJUNITREPORTCMD=${GOJUNIT:-go-junit-report}

TESTS_DIR=${TESTS_DIR:-tests}

cd ${TESTS_DIR}

SED="sed"

PASS_COLOR=$(printf "\033[32mPASS\033[0m")
FAIL_COLOR=$(printf "\033[31mFAIL\033[0m")
TEST_COLORIZE="${SED} 's/PASS/${PASS_COLOR}/' | ${SED} 's/FAIL/${FAIL_COLOR}/'"

mkdir -p results/junit

# -parallel 8 lets all parallel subtests in the correctness packages run at
# once on the 4-vCPU CI runners (default would otherwise be GOMAXPROCS=4).
RUN_TESTBED=1 go test -v -parallel 8 ${TEST_ARGS} 2>&1 | tee results/testoutput.log | bash -c "${TEST_COLORIZE}"

testStatus=${PIPESTATUS[0]}

${GOJUNITREPORTCMD} < results/testoutput.log > results/junit/results.xml

bash -c "cat results/TESTRESULTS.md | ${TEST_COLORIZE}"

exit ${testStatus}
