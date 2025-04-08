// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog

import (
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"time"
)

const (
	timestamp = "timestamp"
	duration  = "duration"

	attributeAWSS3BucketOwner = "aws.s3.owner"
	attributeAWSS3ObjectSize  = "aws.s3.object.size"
	attributeAWSS3TurnAround  = "aws.s3.turn_around"
	attributeAWSS3AclRequired = "aws.s3.acl_required"
)

var attributeNames = [...]string{
	0:  "aws.s3.owner",
	1:  semconv.AttributeAWSS3Bucket,
	2:  timestamp,
	3:  semconv.AttributeSourceAddress,
	4:  semconv.AttributeUserID,
	5:  "aws.s3.request_id",
	6:  semconv.AttributeEventName,
	7:  semconv.AttributeAWSS3Key,
	8:  semconv.AttributeHTTPRequestMethod,
	9:  semconv.AttributeURLOriginal,
	10: semconv.AttributeURLScheme,
	11: semconv.AttributeHTTPResponseStatusCode,
	12: semconv.AttributeErrorType,
	13: semconv.AttributeHTTPResponseBodySize,
	14: attributeAWSS3ObjectSize,
	15: duration,
	16: attributeAWSS3TurnAround,
	17: "http.request.header.referer",
	18: semconv.AttributeUserAgentOriginal,
	19: "aws.s3.version_id",
	20: semconv.AttributeHostID,
	21: "aws.signature.version",
	22: semconv.AttributeTLSCipher,
	23: "aws.s3.auth_type",
	24: "http.request.header.host",
	25: semconv.AttributeTLSProtocolVersion,
	26: "aws.s3.access_point.arn",
	27: attributeAWSS3AclRequired,
}

var strToMonth = map[string]time.Month{
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
