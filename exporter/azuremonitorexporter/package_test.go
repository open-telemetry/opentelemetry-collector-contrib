// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"

	"go.uber.org/goleak"
)

// Context for ignore opencensus leak: https://github.com/census-instrumentation/opencensus-go/issues/1191
// Context for ignore ApplicationInsights-Go leak: https://github.com/microsoft/ApplicationInsights-Go/issues/70
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/microsoft/ApplicationInsights-Go/appinsights.(*throttleManager).Stop"),
	)
}
