package azureeventhubsexporter
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package azureeventhubsexporter exports OpenTelemetry telemetry data to Azure Event Hubs.
//
// This package provides an OpenTelemetry Collector exporter that sends traces, metrics,
// and logs to Azure Event Hubs. The exporter supports various authentication methods
// including connection strings, service principals, and managed identities.
//
// The exporter supports JSON and Protocol Buffer formats for telemetry data and provides
// configurable partition key strategies for optimal load distribution across Event Hub partitions.
package azureeventhubsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter"
