// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Bogus error not to be merged.
	// Fail misspell with langauge as a typo
	goleak.VerifyTestMain(m)
}
