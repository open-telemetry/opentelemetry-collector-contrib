// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributes // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsapplicationsignalsprocessor/internal/attributes"

const (
	// aws attributes
	AWSSpanKind                 = "aws.span.kind"
	AWSLocalService             = "aws.local.service"
	AWSLocalEnvironment         = "aws.local.environment"
	AWSLocalOperation           = "aws.local.operation"
	AWSRemoteService            = "aws.remote.service"
	AWSRemoteOperation          = "aws.remote.operation"
	AWSRemoteEnvironment        = "aws.remote.environment"
	AWSRemoteTarget             = "aws.remote.target"
	AWSRemoteResourceIdentifier = "aws.remote.resource.identifier"
	AWSRemoteResourceType       = "aws.remote.resource.type"
	AWSHostedInEnvironment      = "aws.hostedin.environment"

	// resource detection processor attributes
	ResourceDetectionHostId   = "host.id"
	ResourceDetectionHostName = "host.name"
	ResourceDetectionASG      = "ec2.tag.aws:autoscaling:groupName"
)
