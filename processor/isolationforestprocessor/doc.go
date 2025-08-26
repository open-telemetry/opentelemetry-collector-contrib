// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package isolationforestprocessor provides an OpenTelemetry Collector processor
// that uses the isolation forest machine learning algorithm for anomaly detection
// in telemetry data.
//
// The isolation forest algorithm works by isolating anomalies instead of profiling
// normal data points. It builds an ensemble of isolation trees that randomly
// partition the data space. Anomalous points require fewer partitions to isolate
// than normal points, making them easier to identify.
//
// This processor can operate on traces, metrics, and logs, extracting numerical
// features from telemetry attributes and applying the isolation forest algorithm
// to compute anomaly scores. The processor can either enrich data with anomaly
// scores or filter data based on anomaly thresholds.
//
// Key features:
//   - Unsupervised anomaly detection (no training labels required)
//   - Multi-dimensional analysis across telemetry attributes
//   - Real-time processing with configurable model updates
//   - Support for multiple models based on attribute selectors
//   - Configurable feature extraction for different signal types
//
// The processor follows OpenTelemetry Collector patterns and can be integrated
// into existing telemetry pipelines with minimal configuration changes.
package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"
