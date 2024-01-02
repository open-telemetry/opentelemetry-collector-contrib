// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:generate mdatagen metadata.yaml

// Package awsemfexporter implements an OpenTelemetry Collector exporter that sends EmbeddedMetricFormat to
// AWS CloudWatch Logs in the region the collector is running in using the PutLogEvents API.
package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
