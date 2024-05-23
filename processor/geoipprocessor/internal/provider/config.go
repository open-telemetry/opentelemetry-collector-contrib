// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package provider // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"

import "go.opentelemetry.io/collector/component"

// TODO: add common functionalities (e.g. set root path?)
type Config interface {
	component.Config
}
