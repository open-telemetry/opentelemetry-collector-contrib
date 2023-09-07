// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"testing"

	"go.uber.org/zap"
)

func TestSetInvalidError(_ *testing.T) {
	printErrors(zap.NewExample(), nil) // you know, for coverage
	// The actual test cen be found in the following locations:
	//
	// exporter_test.go where we filter logs by error scope
}
