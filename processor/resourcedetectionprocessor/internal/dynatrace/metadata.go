// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/dynatrace"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
)

type dummyFactory struct{}

func (dummyFactory) Type() component.Type {
	return metadata.Type
}
func (dummyFactory) CreateDefaultConfig() component.Config {
	return nil
}

// Necessary to satisfy mdatagen tests
func NewFactory() component.Factory {
	return dummyFactory{}
}
