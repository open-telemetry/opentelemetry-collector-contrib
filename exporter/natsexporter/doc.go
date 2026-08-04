// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package natsexporter exports OpenTelemetry traces, metrics and logs to a
// NATS server, either via core NATS publish/subscribe or JetStream.
package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"
