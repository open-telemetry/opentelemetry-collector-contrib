// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter"

import "errors"

var (
	// errMissingToken indicates that the Tinybird API token is missing.
	errMissingToken = errors.New("missing Tinybird API token")

	// errMissingEndpoint indicates that the Tinybird API endpoint is missing.
	errMissingEndpoint = errors.New("missing Tinybird API endpoint")
)
