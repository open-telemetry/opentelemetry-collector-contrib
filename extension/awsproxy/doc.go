// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package awsproxy defines an extension that accepts requests without any authentication of AWS signatures
// applied and forwards them to the AWS API, applying authentication and signing.
package awsproxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"
