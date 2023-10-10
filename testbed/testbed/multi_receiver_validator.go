// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

// MultiReceiverValidator defines the interface for validating and reporting test results.
type MultiReceiverTestCaseValidator struct {
	// Validate executes validation routines and test assertions.
	Validate func(tc *MultiReceiverTestCase)

	dataProvider      DataProvider
	assertionFailures []*TraceAssertionFailure
}


func NewMultiReceiverTestCaseValidator(
  senderName string, 
  provider DataProvider,
  validator func(tc *MultiReceiverTestCase),
) *MultiReceiverTestCaseValidator {
	return &MultiReceiverTestCaseValidator{
		dataProvider: provider,
    Validate: validator,
	}
}

func (v *MultiReceiverTestCaseValidator) RecordResults(tc *MultiReceiverTestCase) {
	var result string
	if tc.T.Failed() {
		result = "FAIL"
	} else {
		result = "PASS"
	}

	// Remove "Test" prefix from test name.
	testName := tc.T.Name()[4:]
	tc.ResultsSummary.Add(tc.T.Name(), &MultiReceiverTestResult{
		testName:                   testName,
		result:                     result,
		traceAssertionFailureCount: uint64(len(v.assertionFailures)),
		traceAssertionFailures:     v.assertionFailures,
	})
}
