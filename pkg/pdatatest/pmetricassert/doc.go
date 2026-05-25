// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package pmetricassert provides an MTS-focused assertion framework for
// pmetric.Metrics, based on an editable YAML assertion snapshot.
//
// Unlike pmetrictest.CompareMetrics, which compares a full pdata tree and
// opts out of specific fields, pmetricassert starts from an identity-only
// snapshot (resource attributes, scope identity, metric metadata, datapoint
// attributes) and lets the test opt into additional fields (values,
// timestamps, exemplars).
// Attribute keys may use /exists when the key must be present but its value is
// volatile. Attribute maps may use attributes/include instead of attributes to
// assert a subset of the map while allowing extra keys.
//
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/48079
// for the design discussion and roadmap of operator-suffix grammar
// extensions.
package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
