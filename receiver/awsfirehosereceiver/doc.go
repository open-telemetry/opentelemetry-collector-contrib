// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package awsfirehosereceiver implements a receiver that can be used to
// receive requests from the AWS Kinesis Data Firehose and transform them
// into formats usable by the Opentelemetry collector. The configuration
// determines which unmarshaler to use. Each unmarshaler is responsible for
// processing a Firehose record format that can be sent through the delivery
// stream.
//
// More details can be found at:
// https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html

//go:generate mdatagen metadata.yaml

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"
