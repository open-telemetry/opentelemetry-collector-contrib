// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsxray

// AWS-specific OpenTelemetry attribute names
const (
	AWSOperationAttribute = "aws.operation"
	AWSAccountAttribute   = "aws.account_id"
	AWSRegionAttribute    = "aws.region"
	AWSRequestIDAttribute = "aws.request_id"
	// Currently different instrumentation uses different tag formats.
	// TODO(anuraaga): Find current instrumentation and consolidate.
	AWSRequestIDAttribute2 = "aws.requestId"
	AWSQueueURLAttribute   = "aws.queue_url"
	AWSQueueURLAttribute2  = "aws.queue.url"
	AWSServiceAttribute    = "aws.service"
	AWSTableNameAttribute  = "aws.table_name"
	AWSTableNameAttribute2 = "aws.table.name"

	// AWSXRayInProgressAttribute is the `in_progress` flag in an X-Ray segment
	AWSXRayInProgressAttribute = "aws.xray.inprogress"

	// AWSXRayXForwardedForAttribute is the `x_forwarded_for` flag in an X-Ray segment
	AWSXRayXForwardedForAttribute = "aws.xray.x_forwarded_for"

	// AWSXRayResourceARNAttribute is the `resource_arn` field in an X-Ray segment
	AWSXRayResourceARNAttribute = "aws.xray.resource_arn"

	// AWSXRayTracedAttribute is the `traced` field in an X-Ray subsegment
	AWSXRayTracedAttribute = "aws.xray.traced"

	// AWSXraySegmentMetadataAttributePrefix is the prefix of the attribute that
	// will be treated by the X-Ray exporter as metadata. The key of a metadata
	// will be AWSXraySegmentMetadataAttributePrefix + <metadata_key>.
	AWSXraySegmentMetadataAttributePrefix = "aws.xray.metadata."

	// AWSXrayRetriesAttribute is the `retries` field in an X-Ray (sub)segment.
	AWSXrayRetriesAttribute = "aws.xray.retries"

	// AWSXrayExceptionIDAttribute is the `id` field in an exception
	AWSXrayExceptionIDAttribute = "aws.xray.exception.id"
	// AWSXrayExceptionRemoteAttribute is the `remote` field in an exception
	AWSXrayExceptionRemoteAttribute = "aws.xray.exception.remote"
	// AWSXrayExceptionTruncatedAttribute is the `truncated` field in an exception
	AWSXrayExceptionTruncatedAttribute = "aws.xray.exception.truncated"
	// AWSXrayExceptionSkippedAttribute is the `skipped` field in an exception
	AWSXrayExceptionSkippedAttribute = "aws.xray.exception.skipped"
	// AWSXrayExceptionCauseAttribute is the `cause` field in an exception
	AWSXrayExceptionCauseAttribute = "aws.xray.exception.cause"
)
