// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/elb-access-log"

const (
	AttributeELBStatusCode         = "aws.elb.status.code"              // int
	AttributeELBBackendStatusCode  = "aws.elb.backend.status.code"      // int
	AttributeTLSListenerResourceID = "aws.elb.tls.listener.resource_id" // string
)
