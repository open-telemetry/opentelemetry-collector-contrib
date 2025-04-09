// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/s3-access-log"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
)

const (
	// any field can be set to - to indicate that the data was unknown
	// or unavailable, or that the field was not applicable to this request.
	//
	// See https://docs.aws.amazon.com/AmazonS3/latest/userguide/LogFormat.html.
	unknownField = "-"
)

type s3AccessLogUnmarshaler struct {
	buildInfo component.BuildInfo
}

var _ plog.Unmarshaler = (*s3AccessLogUnmarshaler)(nil)

func NewS3AccessLogUnmarshaler(buildInfo component.BuildInfo) plog.Unmarshaler {
	return &s3AccessLogUnmarshaler{
		buildInfo: buildInfo,
	}
}

type resourceAttributes struct {
	bucketOwner string
	bucketName  string
}

func (s *s3AccessLogUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	scanner := bufio.NewScanner(bytes.NewReader(buf))

	logs, resourceLogs, scopeLogs := s.createLogs()
	resourceAttr := &resourceAttributes{}
	for scanner.Scan() {
		log := scanner.Text()
		if err := handleLog(resourceAttr, scopeLogs, log); err != nil {
			return plog.Logs{}, err
		}
	}

	if err := scanner.Err(); err != nil {
		return plog.Logs{}, fmt.Errorf("error reading log line: %w", err)
	}

	s.setResourceAttributes(resourceAttr, resourceLogs)
	return logs, nil
}

// createLogs with the expected fields for the scope logs
func (s *s3AccessLogUnmarshaler) createLogs() (plog.Logs, plog.ResourceLogs, plog.ScopeLogs) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(s.buildInfo.Version)
	return logs, resourceLogs, scopeLogs
}

// setResourceAttributes based on the resourceAttributes
func (s *s3AccessLogUnmarshaler) setResourceAttributes(r *resourceAttributes, logs plog.ResourceLogs) {
	attr := logs.Resource().Attributes()
	attr.PutStr(semconv.AttributeCloudProvider, semconv.AttributeCloudProviderAWS)
	if r.bucketName != "" {
		attr.PutStr(semconv.AttributeAWSS3Bucket, r.bucketName)
	}
	if r.bucketOwner != "" {
		attr.PutStr(attributeAWSS3BucketOwner, r.bucketOwner)
	}
}

// removeQuotes removes at most two quotes from original
func removeQuotes(original string) (string, error) {
	value1, remaining, found := strings.Cut(original, `"`)
	if !found {
		return original, nil
	}

	value2, remaining, found := strings.Cut(remaining, `"`)
	if !found {
		if value1 != "" {
			return value1, nil
		}
		return value2, nil
	}
	if remaining != "" {
		return "", fmt.Errorf("unexpected: %q has data after the second quote", original)
	}
	return value2, nil
}

func handleLog(resourceAttr *resourceAttributes, scopeLogs plog.ScopeLogs, log string) error {
	record := plog.NewLogRecord()

	remaining := log
	var i int
	var value string
	var err error
	for i = 0; remaining != ""; i++ {
		if i >= len(attributeNames) {
			return errors.New("values in log line exceed the number of available fields")
		}

		value, remaining, _ = strings.Cut(remaining, " ")
		// Values coming from log parsing can contain up to two quotes.
		// The values that contain just one quote contrary to none or
		// two quotes, are the ones coming from splitting the request
		// URI field.
		// Example:
		// Input: "GET /amzn-s3-demo-bucket1?versioning HTTP/1.1"
		// Output of splitting this:
		//	1: "GET
		//  2: /amzn-s3-demo-bucket1?versioning
		//  3: HTTP/1.1"
		if value, err = removeQuotes(value); err != nil {
			return err
		}

		if value == unknownField && attributeNames[i] != attributeAWSS3AclRequired {
			// acl required field can be '-' to indicate that no ACL was required
			if i == 8 {
				// The request uri is unknown, so we skip
				// the next two expected fields in the attributeNames
				// map: url path and scheme.
				i += 2
			}
			continue
		}

		if i == 2 {
			// This is the timestamp that follows a strict format
			// "[DD/MM/YYYY:HH:mm:ss zone]". Since zone is after a
			// space, we cut the string again to remove the zone.
			// Zone is always UTC, so it will always be +0000.
			_, remaining, _ = strings.Cut(remaining, " ")
		}

		if err = addField(attributeNames[i], value, resourceAttr, record); err != nil {
			return err
		}
	}

	if i != len(attributeNames) {
		return errors.New("values in log line are less than the number of available fields")
	}

	rScope := scopeLogs.LogRecords().AppendEmpty()
	record.MoveTo(rScope)

	return nil
}

func addField(field string, value string, resourceAttr *resourceAttributes, record plog.LogRecord) error {
	switch field {
	case attributeAWSS3BucketOwner:
		resourceAttr.bucketOwner = value
	case semconv.AttributeAWSS3Bucket:
		resourceAttr.bucketName = value
	case timestamp:
		// The format in S3 access logs at this point is as "[DD/MM/YYYY:HH:mm:ss".
		t, err := time.Parse("02/Jan/2006:15:04:05", value[1:])
		if err != nil {
			return fmt.Errorf("failed to get timestamp of log: %w", err)
		}
		record.SetTimestamp(pcommon.NewTimestampFromTime(t))
	case semconv.AttributeHTTPResponseStatusCode,
		semconv.AttributeHTTPResponseBodySize,
		attributeAWSS3ObjectSize,
		attributeAWSS3TurnAroundTime,
		duration:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("value for field %q in log line is not a number", field)
		}
		record.Attributes().PutInt(field, n)
	case attributeAWSS3AclRequired:
		// If the request required an ACL for authorization, the string is Yes.
		// If no ACLs were required, the string is -.
		switch value {
		case "Yes":
			record.Attributes().PutBool(field, true)
		case unknownField:
			record.Attributes().PutBool(field, false)
		default:
			return fmt.Errorf("unknown value %q for field %q", value, field)
		}
	case semconv.AttributeTLSProtocolVersion:
		// The value is one of following: TLSv1.1, TLSv1.2, TLSv1.3.
		// We get the version after "v".
		_, remaining, _ := strings.Cut(value, "v")
		if remaining == "" {
			_, remaining, _ = strings.Cut(value, "V")
			if remaining == "" {
				return fmt.Errorf("unexpected TLS version: %q", value)
			}
		}
		record.Attributes().PutStr(field, remaining)
	default:
		record.Attributes().PutStr(field, value)
	}
	return nil
}
