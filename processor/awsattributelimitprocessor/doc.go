// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package awsattributelimitprocessor implements an OTel metrics processor that
// enforces the aws backend 150-attribute limit per metric using
// a three-step approach: unconditional removal of configurable redundant attributes,
// priority-based tier dropping, and force-pruning as a last resort.
package awsattributelimitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsattributelimitprocessor"
