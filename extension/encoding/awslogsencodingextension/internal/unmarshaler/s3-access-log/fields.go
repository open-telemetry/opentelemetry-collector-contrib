// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/s3-access-log"

import (
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

const (
	duration = "duration"

	attributeAWSS3BucketOwner    = "aws.s3.owner"
	attributeAWSS3ObjectSize     = "aws.s3.object.size"
	attributeAWSS3TurnAroundTime = "aws.s3.turn_around_time"
	attributeAWSS3AclRequired    = "aws.s3.acl_required"

	fieldIndexS3BucketOwner    = 0
	fieldIndexS3BucketName     = 1
	fieldIndexTime             = 2
	fieldIndexSourceAddress    = 3
	fieldIndexRequester        = 4
	fieldIndexRequestID        = 5
	fieldIndexOperation        = 6
	fieldIndexS3Key            = 7
	fieldIndexRequestURI       = 8
	fieldIndexHTTPStatus       = 9
	fieldIndexErrorCode        = 10
	fieldIndexBytesSent        = 11
	fieldIndexObjectSize       = 12
	fieldIndexTotalTime        = 13
	fieldIndexTurnAroundTime   = 14
	fieldIndexReferer          = 15
	fieldIndexUserAgent        = 16
	fieldIndexVersionID        = 17
	fieldIndexHostID           = 18
	fieldIndexSignatureVersion = 19
	fieldIndexTLSCipher        = 20
	fieldIndexAuthType         = 21
	fieldIndexHostHeader       = 22
	fieldIndexTLSVersion       = 23
	fieldIndexAccessPointARN   = 24
	fieldIndexACLRequired      = 25
)

// Some of the attribute names are based on semantic conventions for AWS S3.
// See https://github.com/open-telemetry/semantic-conventions/blob/main/docs/object-stores/s3.md.
//
// HTTP attributes are based on HTTP semantic conventions.
// See https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-spans.md.
//
// attributeNames maps each available field in the S3 access log to an attribute name. There is
// a comment in front of each attribute to help the reader navigate the mapping.
// See available fields: https://docs.aws.amazon.com/AmazonS3/latest/userguide/LogFormat.html.
var attributeNames = [...]string{
	fieldIndexSourceAddress:    string(conventions.SourceAddressKey),          // remote IP
	fieldIndexRequester:        string(conventions.UserIDKey),                 // requester
	fieldIndexRequestID:        "aws.request_id",                              // request ID
	fieldIndexOperation:        string(conventions.RPCMethodKey),              // operation
	fieldIndexS3Key:            string(conventions.AWSS3KeyKey),               // key
	fieldIndexHTTPStatus:       string(conventions.HTTPResponseStatusCodeKey), // HTTP status
	fieldIndexErrorCode:        string(conventions.ErrorTypeKey),              // error code
	fieldIndexBytesSent:        string(conventions.HTTPResponseBodySizeKey),   // bytes sent
	fieldIndexObjectSize:       attributeAWSS3ObjectSize,                      // object size
	fieldIndexTotalTime:        duration,                                      // total time
	fieldIndexTurnAroundTime:   attributeAWSS3TurnAroundTime,                  // turn around time
	fieldIndexReferer:          "http.request.header.referer",                 // referer
	fieldIndexUserAgent:        string(conventions.UserAgentOriginalKey),      // user agent
	fieldIndexVersionID:        "aws.s3.version_id",                           // version ID
	fieldIndexHostID:           "aws.extended_request_id",                     // host ID
	fieldIndexSignatureVersion: "aws.signature.version",                       // signature version
	fieldIndexTLSCipher:        string(conventions.TLSCipherKey),              // cipher suite
	fieldIndexAuthType:         "aws.s3.auth_type",                            // authentication type
	fieldIndexHostHeader:       "http.request.header.host",                    // host header
	fieldIndexTLSVersion:       string(conventions.TLSProtocolVersionKey),     // TLS version
	fieldIndexAccessPointARN:   "aws.s3.access_point.arn",                     // access point ARN
	fieldIndexACLRequired:      attributeAWSS3AclRequired,                     // acl required
}
