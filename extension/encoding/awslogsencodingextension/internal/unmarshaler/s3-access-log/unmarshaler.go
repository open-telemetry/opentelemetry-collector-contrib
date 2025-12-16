// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/s3-access-log"

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
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

func NewS3AccessLogUnmarshaler(buildInfo component.BuildInfo) unmarshaler.AWSUnmarshaler {
	return &s3AccessLogUnmarshaler{
		buildInfo: buildInfo,
	}
}

type resourceAttributes struct {
	bucketOwner string
	bucketName  string
}

func (s *s3AccessLogUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	scanner := bufio.NewScanner(reader)

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
	scopeLogs.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatS3AccessLog)
	return logs, resourceLogs, scopeLogs
}

// setResourceAttributes based on the resourceAttributes
func (*s3AccessLogUnmarshaler) setResourceAttributes(r *resourceAttributes, logs plog.ResourceLogs) {
	attr := logs.Resource().Attributes()
	attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	if r.bucketName != "" {
		attr.PutStr(string(conventions.AWSS3BucketKey), r.bucketName)
	}
	if r.bucketOwner != "" {
		attr.PutStr(attributeAWSS3BucketOwner, r.bucketOwner)
	}
}

// scanField gets the next value in the log line by moving
// one space. If the value starts with a quote, it moves
// forward until it finds the ending quote, and considers
// that just 1 value. Otherwise, it returns the value as
// it is.
func scanField(logLine string) (string, string, error) {
	if logLine == "" {
		return "", "", io.EOF
	}

	if logLine[0] != '"' {
		value, remaining, _ := strings.Cut(logLine, " ")
		return value, remaining, nil
	}

	// if there is a quote, we need to get the rest of the value
	logLine = logLine[1:] // remove first quote
	value, remaining, found := strings.Cut(logLine, `"`)
	if !found {
		return "", "", fmt.Errorf("value %q has no end quote", logLine)
	}

	// Remove space after closing quote if present
	if remaining != "" && remaining[0] == ' ' {
		remaining = remaining[1:]
	}

	return value, remaining, nil
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

		value, remaining, err = scanField(remaining)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		if value == unknownField && i != fieldIndexACLRequired {
			// acl required field can be '-' to indicate that no ACL was required
			continue
		}

		if i == fieldIndexTime {
			// This is the timestamp that follows a strict format
			// "[DD/MM/YYYY:HH:mm:ss zone]". Since zone is after a
			// space, we cut the string again to remove the zone.
			var zone string
			zone, remaining, _ = strings.Cut(remaining, " ")
			value = value + " " + zone
		}

		if err := addField(i, value, resourceAttr, record); err != nil {
			return err
		}
	}

	if i != fieldIndexACLRequired+1 {
		return errors.New("values in log line are less than the number of available fields")
	}

	rScope := scopeLogs.LogRecords().AppendEmpty()
	record.MoveTo(rScope)

	return nil
}

func addField(field int, value string, resourceAttr *resourceAttributes, record plog.LogRecord) error {
	switch field {
	case fieldIndexS3BucketOwner:
		resourceAttr.bucketOwner = value
	case fieldIndexS3BucketName:
		resourceAttr.bucketName = value
	case fieldIndexTime:
		t, err := time.Parse("[02/Jan/2006:15:04:05 -0700]", value)
		if err != nil {
			return fmt.Errorf("failed to get timestamp of log: %w", err)
		}
		record.SetTimestamp(pcommon.NewTimestampFromTime(t))
	case fieldIndexHTTPStatus,
		fieldIndexBytesSent,
		fieldIndexObjectSize,
		fieldIndexTurnAroundTime,
		fieldIndexTotalTime:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("value for field %q in log line is not a number", field)
		}
		attrName := attributeNames[field]
		record.Attributes().PutInt(attrName, n)
	case fieldIndexACLRequired:
		attrName := attributeNames[field]
		// If the request required an ACL for authorization, the string is Yes.
		// If no ACLs were required, the string is -.
		switch value {
		case "Yes":
			record.Attributes().PutBool(attrName, true)
		case unknownField:
			record.Attributes().PutBool(attrName, false)
		default:
			return fmt.Errorf("unknown value %q for field %q", value, field)
		}
	case fieldIndexTLSVersion:
		// The value is one of following: TLSv1.1, TLSv1.2, TLSv1.3.
		i := strings.IndexRune(value, '1')
		if i == -1 {
			return fmt.Errorf("missing TLS version: %q", value)
		}
		tlsVersion := value[i:]
		attrName := attributeNames[fieldIndexTLSVersion]
		record.Attributes().PutStr(attrName, tlsVersion)
	case fieldIndexRequestURI:
		method, remaining, _ := strings.Cut(value, " ")
		if method == "" {
			return fmt.Errorf("unexpected: request uri %q has no method", value)
		}
		record.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), method)

		requestURI, remaining, _ := strings.Cut(remaining, " ")
		if requestURI == "" {
			return fmt.Errorf("unexpected: request uri %q has no request URI", value)
		}
		res, err := url.ParseRequestURI(requestURI)
		if err != nil {
			return fmt.Errorf("request uri path is invalid: %w", err)
		}
		if res.Path != "" {
			record.Attributes().PutStr(string(conventions.URLPathKey), res.Path)
		}
		if res.RawQuery != "" {
			record.Attributes().PutStr(string(conventions.URLQueryKey), res.RawQuery)
		}
		if res.Scheme != "" {
			record.Attributes().PutStr(string(conventions.URLSchemeKey), res.Scheme)
		}

		protocol, remaining, _ := strings.Cut(remaining, " ")
		if protocol == "" {
			return fmt.Errorf("unexpected: request uri %q has no protocol", value)
		}
		if remaining != "" {
			return fmt.Errorf(`request uri %q does not have expected format "<method> <path> <protocol>"`, value)
		}
		name, version, err := netProtocol(protocol)
		if err != nil {
			return err
		}
		record.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), name)
		record.Attributes().PutStr(string(conventions.NetworkProtocolVersionKey), version)
	default:
		attrName := attributeNames[field]
		record.Attributes().PutStr(attrName, value)
	}
	return nil
}

// netProtocol returns protocol name and version based on proto value
func netProtocol(proto string) (string, string, error) {
	name, version, found := strings.Cut(proto, "/")
	if !found || name == "" || version == "" {
		return "", "", errors.New(`request uri protocol does not follow expected scheme "<name>/<version>"`)
	}
	switch name {
	case "HTTP":
		name = "http"
	case "QUIC":
		name = "quic"
	case "SPDY":
		name = "spdy"
	default:
		name = strings.ToLower(name)
	}
	return name, version, nil
}
