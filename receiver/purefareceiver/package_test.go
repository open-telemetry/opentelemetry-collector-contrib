// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefareceiver

import (
	"testing"

	"go.uber.org/goleak"
)

// Regarding the OpenCensus ignore: see https://github.com/census-instrumentation/opencensus-go/issues/1191
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
}
