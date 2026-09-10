// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension

import (
	"os"
	"testing"

	"go.uber.org/goleak"
)

func setupTestMain(m *testing.M) {
	_ = os.RemoveAll("test-storage")
	goleak.VerifyTestMain(m, goleak.Cleanup(func(code int) {
		_ = os.RemoveAll("test-storage")
		os.Exit(code)
	}))
}
