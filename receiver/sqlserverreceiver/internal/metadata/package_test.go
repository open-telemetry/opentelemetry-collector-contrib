// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Bogus change not to be merged.
	goleak.VerifyTestMain(m)
}
