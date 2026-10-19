// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package coredistro identifies the core distribution (otelcol) from opentelemetry-collector-releases.
package coredistro // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/coredistro"

import (
	"go.opentelemetry.io/collector/component"
)

// RemovalWarning starts the warning logged by components scheduled for removal from the core distribution.
const RemovalWarning = "This component will be removed from the OpenTelemetry Collector core distribution (otelcol)"

// TransformFilterCoda is the coda for components superseded by the transform and filter processors.
const TransformFilterCoda = "Use the transform or filter processor instead."

// IsCoreDistro reports whether the running collector is the core distribution.
func IsCoreDistro(info component.BuildInfo) bool {
	return info.Command == "otelcol" && info.Description == "OpenTelemetry Collector"
}

// WarnRemoval logs RemovalWarning with the removal version and coda if the running collector is the core distribution.
func WarnRemoval(set component.TelemetrySettings, info component.BuildInfo, version, coda string) {
	if IsCoreDistro(info) {
		set.Logger.Warn(RemovalWarning + " in " + version + ". " + coda)
	}
}
