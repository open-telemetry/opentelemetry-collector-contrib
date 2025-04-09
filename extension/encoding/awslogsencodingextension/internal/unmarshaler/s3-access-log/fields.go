// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/s3-access-log"

import (
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
)

const (
	timestamp = "timestamp"
	duration  = "duration"

	requestURI = "request.uri"

	attributeAWSS3BucketOwner    = "aws.s3.owner"
	attributeAWSS3ObjectSize     = "aws.s3.object.size"
	attributeAWSS3TurnAroundTime = "aws.s3.turn_around_time"
	attributeAWSS3AclRequired    = "aws.s3.acl_required"
)

// Some of the attribute names are based on semantic conventions for AWS S3.
// See https://github.com/open-telemetry/semantic-conventions/blob/main/docs/object-stores/s3.md.
//
// HTTP attributes are based on HTTP semantic conventions.
// See https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-spans.md.
//
// attributeNames maps each available field in the S3 access log to an attribute name.
// See available fields: https://docs.aws.amazon.com/AmazonS3/latest/userguide/LogFormat.html.
var attributeNames = [...]string{
	0:  attributeAWSS3BucketOwner,               // bucket owner
	1:  semconv.AttributeAWSS3Bucket,            // bucket name
	2:  timestamp,                               // time
	3:  semconv.AttributeSourceAddress,          // remote IP
	4:  semconv.AttributeUserID,                 // requester
	5:  "aws.request_id",                        // request ID
	6:  semconv.AttributeRPCMethod,              // operation
	7:  semconv.AttributeAWSS3Key,               // key
	8:  requestURI,                              // request URI, splits in 3 for 3 different attributes
	9:  semconv.AttributeHTTPResponseStatusCode, // HTTP status
	10: semconv.AttributeErrorType,              // error code
	11: semconv.AttributeHTTPResponseBodySize,   // bytes sent
	12: attributeAWSS3ObjectSize,                // object size
	13: duration,                                // total time
	14: attributeAWSS3TurnAroundTime,            // turn around time
	15: "http.request.header.referer",           // referer
	16: semconv.AttributeUserAgentOriginal,      // user agent
	17: "aws.s3.version_id",                     // version ID
	18: "aws.extended_request_id",               // host ID
	19: "aws.signature.version",                 // signature version
	20: semconv.AttributeTLSCipher,              // cypher suite
	21: "aws.s3.auth_type",                      // authentication type
	22: "http.request.header.host",              // host header
	23: semconv.AttributeTLSProtocolVersion,     // TLS version
	24: "aws.s3.access_point.arn",               // access point ARN
	25: attributeAWSS3AclRequired,               // acl required
}
