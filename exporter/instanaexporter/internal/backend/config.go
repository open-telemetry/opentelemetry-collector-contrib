// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package backend // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/backend"

const (
	// AttributeInstanaHostID can be used to distinguish multiple hosts' data
	// being processed by a single collector (in a chained scenario)
	AttributeInstanaHostID = "instana.host.id"

	HeaderKey  = "x-instana-key"
	HeaderHost = "x-instana-host"
	HeaderTime = "x-instana-time"
)
