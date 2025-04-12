// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package attributesprocessor contains the logic to modify attributes of a span.
// It supports insert, update, upsert and delete as actions.
package attributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
