// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// We have to ignore seelog because of upstream issue
		// https://github.com/cihub/seelog/issues/182
		goleak.IgnoreAnyFunction("github.com/cihub/seelog.(*asyncLoopLogger).processQueue"),

		// known goroutine leak http://github.com/DataDog/datadog-agent/issues/40071
		goleak.IgnoreAnyFunction("github.com/DataDog/datadog-agent/pkg/config/viperconfig.(*safeConfig).processNotifications"),
	)
}
