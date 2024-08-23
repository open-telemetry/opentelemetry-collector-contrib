// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure_logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure_logs"

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
