// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/extensions"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

// Factories returns the set of extension factories supported by the OpAMP
// supervisor, keyed by component type.
//
// Extensions are added on a case by case basis as their functionality is wired in.
func Factories() map[component.Type]extension.Factory {
	return map[component.Type]extension.Factory{
		extensiontest.NopType: extensiontest.NewNopFactory(),
	}
}
