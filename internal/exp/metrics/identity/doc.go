// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// identity types for metrics and sample streams.
//
// Use the `Of*(T) -> I` functions to obtain a unique, comparable (==) and
// hashable (map key) identity value I for T.
package identity
