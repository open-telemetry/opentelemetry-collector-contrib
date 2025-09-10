// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package akamai // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/akamai"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/akamai/internal/metadata"
)

type dummyFactory struct{}

func (dummyFactory) Type() component.Type {
	return metadata.Type
}

func (dummyFactory) CreateDefaultConfig() component.Config {
	return struct{}{}
}

// Necessary to satisfy mdatagen tests
func NewFactory() component.Factory {
	return dummyFactory{}
}
