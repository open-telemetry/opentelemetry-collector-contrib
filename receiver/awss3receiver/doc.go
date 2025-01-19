// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package awss3receiver implements a receiver that can be used by the
// Opentelemetry collector to retrieve traces previously stored in S3 by the
// AWS S3 Exporter.
package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"
