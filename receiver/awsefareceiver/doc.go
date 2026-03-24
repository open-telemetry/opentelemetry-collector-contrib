// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package awsefareceiver reads Amazon Elastic Fabric Adapter (EFA) metrics
// from /sys/class/infiniband/*/ports/*/hw_counters on Linux hosts.
package awsefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"
