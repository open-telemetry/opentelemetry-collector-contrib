// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/configschema"

import (
	"go.opentelemetry.io/collector/component"
)

// CfgInfo contains a component config instance, as well as its group name and
// type.
type CfgInfo struct {
	// the component type, e.g. "otlpreceiver.Config"
	Type component.Type
	// an instance of the component's configuration struct
	CfgInstance any
}

// GetCfgInfo accepts a Factories struct, then creates and returns the default
// config for the component specified by the passed-in componentType and
// componentName.
func GetCfgInfo(f component.Factory) (CfgInfo, error) {
	return CfgInfo{
		Type:        f.Type(),
		CfgInstance: f.CreateDefaultConfig(),
	}, nil
}
