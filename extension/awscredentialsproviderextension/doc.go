// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package awscredentialsproviderextension resolves AWS credentials and exposes them to other
// components through the Provider interface. AWS-SDK-based components (e.g. the
// awscloudwatch receiver) reference the extension by ID and use the resolved
// aws.CredentialsProvider for their SDK clients.
package awscredentialsproviderextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscredentialsproviderextension"
