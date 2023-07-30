// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package sigv4authextension implements the `auth.Client` interface.
// This extension provides the Sigv4 process of adding authentication information to AWS API requests sent by HTTP.
// As such, the extension can be used for HTTP based exporters that export to AWS services.
package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
