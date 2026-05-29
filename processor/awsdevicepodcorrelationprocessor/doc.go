// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package awsdevicepodcorrelationprocessor implements a generic OTEL metrics processor
// that correlates device metrics with Kubernetes pod/container metadata using the
// Kubelet Pod Resources API.
package awsdevicepodcorrelationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsdevicepodcorrelationprocessor"
