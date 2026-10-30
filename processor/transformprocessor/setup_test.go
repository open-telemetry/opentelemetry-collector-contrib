// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"testing"

	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func setupTestMain(*testing.M) {
	if err := featuregate.GlobalRegistry().Set(metadata.ProcessorTransformEnableProfileStatementsFeatureGate.ID(), true); err != nil {
		panic(err)
	}
}
