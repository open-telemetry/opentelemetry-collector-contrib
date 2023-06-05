// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import "github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"

type transportChannel interface {
	Send(*contracts.Envelope)
}
