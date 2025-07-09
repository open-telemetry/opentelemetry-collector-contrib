// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// We have to ignore seelog because of upstream issue
	// https://github.com/cihub/seelog/issues/182
	goleak.VerifyTestMain(m,
		goleak.IgnoreAnyFunction("github.com/cihub/seelog.(*asyncLoopLogger).processQueue"),
	)
}
