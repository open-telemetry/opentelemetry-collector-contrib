// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterfactory

import (
	"testing"

	"go.uber.org/goleak"
)

// See https://github.com/census-instrumentation/opencensus-go/issues/1191 for more information on ignore.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
}
