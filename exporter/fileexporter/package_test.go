// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(
		m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),          // gorutine started in init() function of a transitive dependency
		goleak.IgnoreTopFunction("gopkg.in/natefinch/lumberjack%2ev2.(*Logger).millRun"), // upstream issue, see: https://github.com/natefinch/lumberjack/issues/56
	)
}
