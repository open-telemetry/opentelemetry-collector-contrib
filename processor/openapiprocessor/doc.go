// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package openapiprocessor processes traces by matching URL paths against OpenAPI
// specifications and adding url.template and peer.service attributes.
package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"
