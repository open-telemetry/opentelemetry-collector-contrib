// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package awssecretsmanagerauthextension implements an extension offering basic auth
// authentication with credentials sourced from AWS Secrets Manager and rotated in place.
package awssecretsmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension"
