// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/extensions"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
)

// Factories returns the set of extension factories supported by the OpAMP
// supervisor, keyed by component type.
//
// Extensions are added on a case by case basis as their functionality is wired in.
func Factories() map[component.Type]extension.Factory {
	fs := []extension.Factory{
		extensiontest.NewNopFactory(),
		bearertokenauthextension.NewFactory(),
		basicauthextension.NewFactory(),
		oauth2clientauthextension.NewFactory(),
	}
	result := make(map[component.Type]extension.Factory, len(fs))
	for _, f := range fs {
		result[f.Type()] = f
	}
	return result
}
