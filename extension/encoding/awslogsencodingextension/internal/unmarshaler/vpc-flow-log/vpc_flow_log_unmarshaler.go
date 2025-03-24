// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
)

const (
	fileFormatPlainText = "plain-text"
	fileFormatParquet   = "parquet"
)

var (
	supportedVPCFlowLogFileFormat = []string{fileFormatPlainText, fileFormatParquet}

	supportedFieldValues = map[string][]string{
		"action":           {"ACCEPT", "REJECT"},
		"log-status":       {"OK", "NODATA", "SKIPDATA"},
		"tcp-flags":        {"1", "2", "4", "18"},
		"type":             {"IPv4", "IPv6", "EFA"},
		"sublocation-type": {"wavelength", "outpost", "localzone"},
		"pkt-src-aws-service": {
			"AMAZON", "AMAZON_APPFLOW", "AMAZON_CONNECT", "API_GATEWAY", "CHIME_MEETINGS", "CHIME_VOICECONNECTOR",
			"CLOUD9", "CLOUDFRONT", "CODEBUILD", "DYNAMODB", "EBS", "EC2", "EC2_INSTANCE_CONNECT", "GLOBALACCELERATOR",
			"KINESIS_VIDEO_STREAMS", "ROUTE53", "ROUTE53_HEALTHCHECKS", "ROUTE53_HEALTHCHECKS_PUBLISHING",
			"ROUTE53_RESOLVER", "S3", "WORKSPACES_GATEWAYS",
		},
		"pkt-dst-aws-service": {
			"AMAZON", "AMAZON_APPFLOW", "AMAZON_CONNECT", "API_GATEWAY", "CHIME_MEETINGS", "CHIME_VOICECONNECTOR",
			"CLOUD9", "CLOUDFRONT", "CODEBUILD", "DYNAMODB", "EBS", "EC2", "EC2_INSTANCE_CONNECT", "GLOBALACCELERATOR",
			"KINESIS_VIDEO_STREAMS", "ROUTE53", "ROUTE53_HEALTHCHECKS", "ROUTE53_HEALTHCHECKS_PUBLISHING",
			"ROUTE53_RESOLVER", "S3", "WORKSPACES_GATEWAYS",
		},
		"flow-direction": {"ingress", "egress"},
		"traffic-path":   {"1", "2", "3", "4", "5", "6", "7", "8"},
		"reject-reason":  {"BPA"},
	}
)

type vpcFlowLogUnmarshaler struct {
	// VPC flow logs can be sent in plain text
	// or parquet files to S3.
	//
	// See https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-s3-path.html.
	fileFormat string

	// Pool the gzip readers, which are expensive to create.
	gzipPool sync.Pool

	buildInfo component.BuildInfo
}

func NewVPCFlowLogUnmarshaler(format string, buildInfo component.BuildInfo) (plog.Unmarshaler, error) {
	switch format {
	case fileFormatParquet: // valid
	case fileFormatPlainText: // valid
	default:
		return nil, fmt.Errorf(
			"unsupported file fileFormat %q for VPC flow log, expected one of %q",
			format,
			supportedVPCFlowLogFileFormat,
		)
	}
	return &vpcFlowLogUnmarshaler{
		fileFormat: format,
		gzipPool:   sync.Pool{},
		buildInfo:  buildInfo,
	}, nil
}

func (v *vpcFlowLogUnmarshaler) UnmarshalLogs(content []byte) (plog.Logs, error) {
	var errGzipReader error
	gzipReader, ok := v.gzipPool.Get().(*gzip.Reader)
	if !ok {
		gzipReader, errGzipReader = gzip.NewReader(bytes.NewReader(content))
	} else {
		errGzipReader = gzipReader.Reset(bytes.NewReader(content))
	}
	if errGzipReader != nil {
		if gzipReader != nil {
			v.gzipPool.Put(gzipReader)
		}
		return plog.Logs{}, fmt.Errorf("failed to decompress content: %w", errGzipReader)
	}
	defer func() {
		_ = gzipReader.Close()
		v.gzipPool.Put(gzipReader)
	}()

	switch v.fileFormat {
	case fileFormatPlainText:
		return v.unmarshalPlainTextLogs(gzipReader)
	case fileFormatParquet:
		return plog.Logs{}, errors.New("still needs to be implemented")
	default:
		// not possible, prevent by NewVPCFlowLogUnmarshaler
		return plog.Logs{}, nil
	}
}

func (v *vpcFlowLogUnmarshaler) unmarshalPlainTextLogs(reader *gzip.Reader) (plog.Logs, error) {
	scanner := bufio.NewScanner(reader)

	logs := v.createLogs()
	scopeLogs := logs.ResourceLogs().At(0).ScopeLogs().At(0)

	// first line includes the fields
	var fields []string
	if scanner.Scan() {
		firstLine := scanner.Text()
		fields = strings.Split(firstLine, " ")
	}

	for scanner.Scan() {
		line := scanner.Text()
		if err := v.addToLogs(scopeLogs, fields, line); err != nil {
			return plog.Logs{}, err
		}
	}

	if err := scanner.Err(); err != nil {
		return plog.Logs{}, fmt.Errorf("error reading log line: %w", err)
	}

	return logs, nil
}

func (v *vpcFlowLogUnmarshaler) createLogs() plog.Logs {
	logs := plog.NewLogs()

	rl := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)
	sl.Scope().SetVersion(v.buildInfo.Version)

	return logs
}

func (v *vpcFlowLogUnmarshaler) addToLogs(
	scopeLogs plog.ScopeLogs,
	fields []string,
	logLine string,
) error {
	values := strings.Split(logLine, " ")
	nFields := len(fields)
	nValues := len(values)
	if nFields != nValues {
		return fmt.Errorf("expect %d fields per log line, got log line with %d fields", nFields, nValues)
	}

	record := scopeLogs.LogRecords().AppendEmpty()
	for i, field := range fields {
		if err := validateVPCFlowLog(field, values[i]); err != nil {
			return err
		}
		record.Attributes().PutStr(field, values[i])
	}

	return nil
}

func validateVPCFlowLog(field string, value string) error {
	if value == "-" {
		return nil
	}

	supportedValues, ok := supportedFieldValues[field]
	if !ok {
		return nil
	}

	for _, supported := range supportedValues {
		if supported == value {
			return nil
		}
	}

	return fmt.Errorf("value %q is invalid for the %s field, valid values are %s", value, field, supportedValues)
}
