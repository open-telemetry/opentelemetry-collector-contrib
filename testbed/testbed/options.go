// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"time"
)

// ResourceSpec is a resource consumption specification.
type ResourceSpec struct {
	// Percentage of one core the process is expected to consume at most.
	// Test is aborted and failed if consumption during
	// ResourceCheckPeriod exceeds this number. If 0 the CPU
	// consumption is not monitored and does not affect the test result.
	ExpectedMaxCPU uint32

	// Maximum RAM in MiB the process is expected to consume.
	// Test is aborted and failed if consumption exceeds this number.
	// If 0 memory consumption is not monitored and does not affect
	// the test result.
	ExpectedMaxRAM uint32

	// Period during which CPU and RAM of the process are measured.
	// Bigger numbers will result in more averaging of short spikes.
	ResourceCheckPeriod time.Duration

	// The number of consecutive violations necessary to trigger a failure.
	// This is useful for tests which can tolerate transitory violations.
	MaxConsecutiveFailures uint32
}

// isSpecified returns true if any part of ResourceSpec is specified,
// i.e. has non-zero value.
func (rs *ResourceSpec) isSpecified() bool {
	return rs != nil && (rs.ExpectedMaxCPU != 0 || rs.ExpectedMaxRAM != 0)
}

// TestCaseOption defines a TestCase option.
type TestCaseOption func(t *TestCase)

// WithSkipResults disables writing out results file for a TestCase.
func WithSkipResults() TestCaseOption {
	return func(tc *TestCase) {
		tc.skipResults = true
	}
}

// WithResourceLimits sets expected limits for resource consmption.
// Error is signaled if consumption during ResourceCheckPeriod exceeds the limits.
// Limits are modified only for non-zero fields of resourceSpec, all zero-value fields
// fo resourceSpec are ignored and their previous values remain in effect.
func WithResourceLimits(resourceSpec ResourceSpec) TestCaseOption {
	return func(tc *TestCase) {
		if resourceSpec.ExpectedMaxCPU > 0 {
			tc.resourceSpec.ExpectedMaxCPU = resourceSpec.ExpectedMaxCPU
		}
		if resourceSpec.ExpectedMaxRAM > 0 {
			tc.resourceSpec.ExpectedMaxRAM = resourceSpec.ExpectedMaxRAM
		}
		if resourceSpec.ResourceCheckPeriod > 0 {
			tc.resourceSpec.ResourceCheckPeriod = resourceSpec.ResourceCheckPeriod
		}
	}
}

// WithDecision enables our mock backend to behave sporadically
func WithDecisionFunc(decision decisionFunc) TestCaseOption {
	return func(tc *TestCase) {
		tc.decision = decision
	}
}
