// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:generate mdatagen metadata.yaml

// Package awsxrayexporter implements an OpenTelemetry Collector exporter that sends trace data to
// AWS X-Ray in the region the collector is running in using the PutTraceSegments API.
package awsxrayexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"
