// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package awsxrayreceiver implements a receiver that can be used by the
// Opentelemetry collector to receive traces in the AWS X-Ray segment format.
// More details can be found on:
// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
package awsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"
