// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/s3-access-log"

import (
	"time"

	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
)

const (
	timestamp = "timestamp"
	duration  = "duration"

	attributeAWSS3BucketOwner = "aws.s3.owner"
	attributeAWSS3ObjectSize  = "aws.s3.object.size"
	attributeAWSS3TurnAround  = "aws.s3.turn_around"
	attributeAWSS3AclRequired = "aws.s3.acl_required"
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
	8:  semconv.AttributeHTTPRequestMethod,      // request URI - 1st field
	9:  semconv.AttributeURLPath,                // request URI - 2nd field after space
	10: semconv.AttributeURLScheme,              // request URI - 3rd field after space
	11: semconv.AttributeHTTPResponseStatusCode, // HTTP status
	12: semconv.AttributeErrorType,              // error code
	13: semconv.AttributeHTTPResponseBodySize,   // bytes sent
	14: attributeAWSS3ObjectSize,                // object size
	15: duration,                                // total time
	16: attributeAWSS3TurnAround,                // turn around time
	17: "http.request.header.referer",           // referer
	18: semconv.AttributeUserAgentOriginal,      // user agent
	19: "aws.s3.version_id",                     // version ID
	20: "aws.extended_request_id",               // host ID
	21: "aws.signature.version",                 // signature version
	22: semconv.AttributeTLSCipher,              // cypher suite
	23: "aws.s3.auth_type",                      // authentication type
	24: "http.request.header.host",              // host header
	25: semconv.AttributeTLSProtocolVersion,     // TLS version
	26: "aws.s3.access_point.arn",               // access point ARN
	27: attributeAWSS3AclRequired,               // acl required
}

var months = map[string]time.Month{
	"Jan": time.January,
	"Feb": time.February,
	"Mar": time.March,
	"Apr": time.April,
	"May": time.May,
	"Jun": time.June,
	"Jul": time.July,
	"Aug": time.August,
	"Sep": time.September,
	"Oct": time.October,
	"Nov": time.November,
	"Dec": time.December,
}
