// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"testing"

	"go.opentelemetry.io/collector/featuregate"
)

func setupTestMain(m *testing.M) {
	// Disable the feature gate for generated tests to match the expected component type
	// The generated tests expect "awscontainerinsightreceiver" (old type name)
	// When the feature gate is enabled, the factory returns "awscontainerinsight" (new type name)
	// So we disable it here to ensure generated tests pass
	_ = featuregate.GlobalRegistry().Set(useNewTypeNameGate.ID(), false)
	m.Run()
}
