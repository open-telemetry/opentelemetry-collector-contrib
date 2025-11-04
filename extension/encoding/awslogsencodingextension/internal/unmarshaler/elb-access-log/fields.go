// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/elb-access-log"

const (
	AttributeELBStatusCode             = "aws.elb.status.code"              // int
	AttributeELBBackendStatusCode      = "aws.elb.backend.status.code"      // int
	AttributeTLSListenerResourceID     = "aws.elb.tls.listener.resource_id" // string
	AttributeELBRequestProcessingTime  = "aws.elb.request_processing_time"  // float
	AttributeELBResponseProcessingTime = "aws.elb.response_processing_time" // float
	AttributeELBTargetProcessingTime   = "aws.elb.target_processing_time"   // float
	AttributeELBTargetGroupARN         = "aws.elb.target_group_arn"         // string
	AttributeELBChosenCertARN          = "aws.elb.chosen_cert_arn"          // string
	AttributeELBActionsExecuted        = "aws.elb.actions_executed"         // string
	AttributeELBRedirectURL            = "aws.elb.redirect_url"             // string
	AttributeELBErrorReason            = "aws.elb.error_reason"             // string
	AttributeELBClassification         = "aws.elb.classification"           // string
	AttributeELBClassificationReason   = "aws.elb.classification_reason"    // string
	AttributeELBConnectionTraceID      = "aws.elb.connection_trace_id"      // string
	AttributeELBTransformedHost        = "aws.elb.transformed_host"         // string
	AttributeELBTransformedURI         = "aws.elb.transformed_uri"          // string
	AttributeELBRequestTransformStatus = "aws.elb.request_transform_status" // string
	AttributeELBAWSTraceID             = "aws.elb.aws_trace_id"             // string
)
