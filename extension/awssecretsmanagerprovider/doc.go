// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package awssecretsmanagerprovider implements an extension that provides secrets from AWS Secrets Manager
// to other extensions and notifies them when a secret changes.
package awssecretsmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerprovider"
