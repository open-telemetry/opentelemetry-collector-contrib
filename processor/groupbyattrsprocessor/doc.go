// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package groupbyattrsprocessor creates Resources based on specified
// attributes, and groups metrics, log records and spans with matching
// attributes under the corresponding Resource.
package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
