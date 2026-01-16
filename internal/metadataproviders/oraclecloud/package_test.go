// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oraclecloud

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain is the entry point for tests in this package. It enables
// automatic goroutine leak detection after tests complete.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
