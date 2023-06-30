// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// package resourcedetectionprocessor implements a processor
// which can be used to detect resource information from the host,
// in a format that conforms to the OpenTelemetry resource semantic conventions, and append or
// override the resource value in telemetry data with this information.
package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
