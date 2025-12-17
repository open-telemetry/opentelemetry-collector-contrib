// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
)

var (
	supportedVPCFlowLogFileFormat = []string{constants.FileFormatPlainText, constants.FileFormatParquet}
	defaultFormat                 = []string{"version", "account-id", "interface-id", "srcaddr", "dstaddr", "srcport", "dstport", "protocol", "packets", "bytes", "start", "end", "action", "log-status"}
)

type Config struct {
	// fileFormat - VPC flow logs can be sent in plain text or parquet files to S3.
	// See https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-s3-path.html.
	FileFormat string `mapstructure:"file_format"`

	// Format defines the custom field format of the VPC flow logs.
	// When defined, this is used for parsing CloudWatch bound VPC flow logs.
	Format string `mapstructure:"format"`

	// parsedFormat holds the parsed fields from Format.
	parsedFormat []string

	// prevent unkeyed literal initialization
	_ struct{}
}

type vpcFlowLogUnmarshaler struct {
	cfg Config

	buildInfo component.BuildInfo
	logger    *zap.Logger

	// Whether VPC flow start field should use ISO-8601 format
	vpcFlowStartISO8601FormatEnabled bool
}

func NewVPCFlowLogUnmarshaler(
	cfg Config,
	buildInfo component.BuildInfo,
	logger *zap.Logger,
	vpcFlowStartISO8601FormatEnabled bool,
) (unmarshaler.AWSUnmarshaler, error) {
	cfg.parsedFormat = defaultFormat
	if cfg.Format != "" {
		cfg.parsedFormat = strings.Fields(cfg.Format)
		logger.Debug("Using custom format for VPC flow log unmarshaling", zap.Strings("fields", cfg.parsedFormat))
	}

	switch cfg.FileFormat {
	case constants.FileFormatParquet:
		// TODO
		return nil, errors.New("still needs to be implemented")
	case constants.FileFormatPlainText: // valid
	default:
		return nil, fmt.Errorf(
			"unsupported file fileFormat %q for VPC flow log, expected one of %q",
			cfg.FileFormat,
			supportedVPCFlowLogFileFormat,
		)
	}

	return &vpcFlowLogUnmarshaler{
		cfg:                              cfg,
		buildInfo:                        buildInfo,
		logger:                           logger,
		vpcFlowStartISO8601FormatEnabled: vpcFlowStartISO8601FormatEnabled,
	}, nil
}

func (v *vpcFlowLogUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	switch v.cfg.FileFormat {
	case constants.FileFormatPlainText:
		return v.unmarshalPlainTextLogs(reader)
	case constants.FileFormatParquet:
		// TODO
		return plog.Logs{}, errors.New("still needs to be implemented")
	default:
		// not possible, prevent by NewVPCFlowLogUnmarshaler
		return plog.Logs{}, nil
	}
}

func (v *vpcFlowLogUnmarshaler) unmarshalPlainTextLogs(reader io.Reader) (plog.Logs, error) {
	// use buffered reader for efficiency and to avoid any size restrictions
	bufReader := bufio.NewReader(reader)

	b, err := bufReader.ReadByte()
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to read first byte: %w", err)
	}

	err = bufReader.UnreadByte()
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unread first byte: %w", err)
	}

	if b == '{' {
		// Dealing with a JSON logs, so check for CW bound trigger
		return v.fromCloudWatch(v.cfg.parsedFormat, bufReader)
	}

	// This is S3 bound data, so use fromS3
	return v.fromS3(bufReader)
}

// fromS3 expects VPC logs from S3 in plain text format
func (v *vpcFlowLogUnmarshaler) fromS3(reader *bufio.Reader) (plog.Logs, error) {
	var err error
	line, err := reader.ReadString('\n')
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to read first line of VPC logs from S3: %w", err)
	}

	fields := strings.Fields(line)
	logs, resourceLogs, scopeLogs := v.createLogs()
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Reached the end of the file, add the last line and exit
				// EOF is ignored as we have already processed all log lines
				if addLogErr := v.addToLogs(resourceLogs, scopeLogs, fields, strings.TrimSpace(line)); addLogErr != nil {
					return plog.Logs{}, addLogErr
				}
				break
			}

			return plog.Logs{}, fmt.Errorf("error reading VPC logs: %w", err)
		}

		if err := v.addToLogs(resourceLogs, scopeLogs, fields, strings.TrimSpace(line)); err != nil {
			return plog.Logs{}, err
		}
	}

	return logs, nil
}

// fromCloudWatch expects VPC logs from CloudWatch Logs subscription filter trigger
func (v *vpcFlowLogUnmarshaler) fromCloudWatch(fields []string, reader *bufio.Reader) (plog.Logs, error) {
	var cwLog events.CloudwatchLogsData
	err := gojson.NewDecoder(reader).Decode(&cwLog)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal data as cloudwatch logs event: %w", err)
	}

	logs, resourceLogs, scopeLogs := v.createLogs()

	resourceAttrs := resourceLogs.Resource().Attributes()
	resourceAttrs.PutStr(string(conventions.AWSLogGroupNamesKey), cwLog.LogGroup)
	resourceAttrs.PutStr(string(conventions.AWSLogStreamNamesKey), cwLog.LogStream)

	for _, event := range cwLog.LogEvents {
		err := v.addToLogs(resourceLogs, scopeLogs, fields, event.Message)
		if err != nil {
			return plog.Logs{}, err
		}
	}

	return logs, nil
}

// createLogs is a helper to create prefilled plog.Logs, plog.ResourceLogs, plog.ScopeLogs
func (v *vpcFlowLogUnmarshaler) createLogs() (plog.Logs, plog.ResourceLogs, plog.ScopeLogs) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(v.buildInfo.Version)
	scopeLogs.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatVPCFlowLog)
	return logs, resourceLogs, scopeLogs
}

// addToLogs parses the log line and creates
// a new record log. The record log is added
// to the scope logs of the resource identified
// by the resourceKey created from the values.
func (v *vpcFlowLogUnmarshaler) addToLogs(
	resourceLogs plog.ResourceLogs,
	scopeLogs plog.ScopeLogs,
	fields []string,
	logLine string,
) error {
	record := scopeLogs.LogRecords().AppendEmpty()
	addr := &address{}
	for _, field := range fields {
		if logLine == "" {
			return errors.New("log line has less fields than the ones expected")
		}
		var value string
		value, logLine, _ = strings.Cut(logLine, " ")

		if value == "-" {
			// If a field is not applicable or could not be computed for a
			// specific record, the record displays a '-' symbol for that entry.
			//
			// See https://docs.aws.amazon.com/vpc/latest/userguide/flow-log-records.html.
			continue
		}

		if strings.HasPrefix(field, "ecs-") {
			v.logger.Warn("currently there is no support for ECS fields")
			continue
		}

		found, err := v.handleField(field, value, resourceLogs, record, addr)
		if err != nil {
			return err
		}
		if !found {
			v.logger.Warn("field is not an available field for a flow log record",
				zap.String("field", field),
				zap.String("documentation", "https://docs.aws.amazon.com/vpc/latest/userguide/flow-log-records.html"),
			)
		}
	}

	if logLine != "" {
		return errors.New("log line has more fields than the ones expected")
	}

	// Add the address fields with the correct conventions to the log record
	v.handleAddresses(addr, record)
	return nil
}

// handleAddresses creates adds the addresses to the log record
func (v *vpcFlowLogUnmarshaler) handleAddresses(addr *address, record plog.LogRecord) {
	localAddrSet := false
	// see example in
	// https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-records-examples.html#flow-log-example-nat
	if addr.pktSource == "" && addr.source != "" {
		// there is no middle layer, assume "srcaddr" field
		// corresponds to the original source address.
		record.Attributes().PutStr(string(conventions.SourceAddressKey), addr.source)
	} else if addr.pktSource != "" && addr.source != "" {
		record.Attributes().PutStr(string(conventions.SourceAddressKey), addr.pktSource)
		if addr.pktSource != addr.source {
			// srcaddr is the middle layer
			record.Attributes().PutStr(string(conventions.NetworkLocalAddressKey), addr.source)
			localAddrSet = true
		}
	}

	if addr.pktDestination == "" && addr.destination != "" {
		// there is no middle layer, assume "dstaddr" field
		// corresponds to the original destination address.
		record.Attributes().PutStr(string(conventions.DestinationAddressKey), addr.destination)
	} else if addr.pktDestination != "" && addr.destination != "" {
		record.Attributes().PutStr(string(conventions.DestinationAddressKey), addr.pktDestination)
		if addr.pktDestination != addr.destination {
			if localAddrSet {
				v.logger.Warn("unexpected: srcaddr, dstaddr, pkt-srcaddr and pkt-dstaddr are all different")
			}
			// dstaddr is the middle layer
			record.Attributes().PutStr(string(conventions.NetworkLocalAddressKey), addr.destination)
		}
	}
}

// handleField analyzes the given field and it either
// adds its value to the resourceKey or puts the
// field and its value in the attributes map. If the
// field is not recognized, it returns false.
func (v *vpcFlowLogUnmarshaler) handleField(
	field string,
	value string,
	resourceLogs plog.ResourceLogs,
	record plog.LogRecord,
	addr *address,
) (bool, error) {
	// convert string to number
	getNumber := func(value string) (int64, error) {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return -1, fmt.Errorf("%q field in log file is not a number", field)
		}
		return n, nil
	}

	// convert string to number and add the
	// value to an attribute
	addNumber := func(field, value, attrName string) error {
		n, err := getNumber(value)
		if err != nil {
			return fmt.Errorf("%q field in log file is not a number", field)
		}
		record.Attributes().PutInt(attrName, n)
		return nil
	}

	switch field {
	case "srcaddr":
		// handled later
		addr.source = value
	case "pkt-srcaddr":
		// handled later
		addr.pktSource = value
	case "dstaddr":
		// handled later
		addr.destination = value
	case "pkt-dstaddr":
		// handled later
		addr.pktDestination = value

	case "account-id":
		resourceLogs.Resource().Attributes().PutStr(string(conventions.CloudAccountIDKey), value)
	case "vpc-id":
		record.Attributes().PutStr("aws.vpc.id", value)
	case "subnet-id":
		record.Attributes().PutStr("aws.vpc.subnet.id", value)
	case "instance-id":
		record.Attributes().PutStr(string(conventions.HostIDKey), value)
	case "az-id":
		record.Attributes().PutStr("aws.az.id", value)
	case "interface-id":
		// TODO Replace with conventions variable once it becomes available
		record.Attributes().PutStr("network.interface.name", value)
	case "srcport":
		if err := addNumber(field, value, string(conventions.SourcePortKey)); err != nil {
			return false, err
		}
	case "dstport":
		if err := addNumber(field, value, string(conventions.DestinationPortKey)); err != nil {
			return false, err
		}
	case "protocol":
		n, err := getNumber(value)
		if err != nil {
			return false, err
		}
		protocolNumber := int(n)
		if protocolNumber < 0 || protocolNumber >= len(protocolNames) {
			return false, fmt.Errorf("protocol number %d does not have a protocol name", protocolNumber)
		}
		record.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), protocolNames[protocolNumber])
	case "type":
		record.Attributes().PutStr(string(conventions.NetworkTypeKey), strings.ToLower(value))
	case "region":
		resourceLogs.Resource().Attributes().PutStr(string(conventions.CloudRegionKey), value)
	case "flow-direction":
		switch value {
		case "ingress":
			record.Attributes().PutStr(string(conventions.NetworkIODirectionKey), "receive")
		case "egress":
			record.Attributes().PutStr(string(conventions.NetworkIODirectionKey), "transmit")
		default:
			return true, fmt.Errorf("value %s not valid for field %s", value, field)
		}
	case "version":
		if err := addNumber(field, value, "aws.vpc.flow.log.version"); err != nil {
			return false, err
		}
	case "packets":
		if err := addNumber(field, value, "aws.vpc.flow.packets"); err != nil {
			return false, err
		}
	case "bytes":
		if err := addNumber(field, value, "aws.vpc.flow.bytes"); err != nil {
			return false, err
		}
	case "start":
		unixSeconds, err := getNumber(value)
		if err != nil {
			return true, fmt.Errorf("value %s for field %s does not correspond to a valid timestamp", value, field)
		}
		if v.vpcFlowStartISO8601FormatEnabled {
			// New behavior: ISO-8601 format (RFC3339Nano)
			timestamp := time.Unix(unixSeconds, 0).UTC()
			record.Attributes().PutStr("aws.vpc.flow.start", timestamp.Format(time.RFC3339Nano))
		} else {
			// Legacy behavior: Unix timestamp as integer
			record.Attributes().PutInt("aws.vpc.flow.start", unixSeconds)
		}
	case "end":
		unixSeconds, err := getNumber(value)
		if err != nil {
			return true, fmt.Errorf("value %s for field %s does not correspond to a valid timestamp", value, field)
		}
		timestamp := time.Unix(unixSeconds, 0)
		record.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	case "action":
		record.Attributes().PutStr("aws.vpc.flow.action", value)
	case "log-status":
		record.Attributes().PutStr("aws.vpc.flow.status", value)
	case "tcp-flags":
		record.Attributes().PutStr("network.tcp.flags", value)
	case "sublocation-type":
		record.Attributes().PutStr("aws.sublocation.type", value)
	case "sublocation-id":
		record.Attributes().PutStr("aws.sublocation.id", value)
	case "pkt-src-aws-service":
		record.Attributes().PutStr("aws.vpc.flow.source.service", value)
	case "pkt-dst-aws-service":
		record.Attributes().PutStr("aws.vpc.flow.destination.service", value)
	case "traffic-path":
		record.Attributes().PutStr("aws.vpc.flow.traffic_path", value)
	case "ecs-cluster-arn":
		record.Attributes().PutStr(string(conventions.AWSECSClusterARNKey), value)
	case "ecs-cluster-name":
		record.Attributes().PutStr("aws.ecs.cluster.name", value)
	case "ecs-container-instance-arn":
		record.Attributes().PutStr("aws.ecs.container.instance.arn", value)
	case "ecs-container-instance-id":
		record.Attributes().PutStr("aws.ecs.container.instance.id", value)
	case "ecs-container-id":
		record.Attributes().PutStr("aws.ecs.container.id", value)
	case "ecs-second-container-id":
		record.Attributes().PutStr("aws.ecs.second.container.id", value)
	case "ecs-service-name":
		record.Attributes().PutStr("aws.ecs.service.name", value)
	case "ecs-task-definition-arn":
		record.Attributes().PutStr("aws.ecs.task.definition.arn", value)
	case "ecs-task-arn":
		record.Attributes().PutStr(string(conventions.AWSECSTaskARNKey), value)
	case "ecs-task-id":
		record.Attributes().PutStr(string(conventions.AWSECSTaskIDKey), value)
	case "reject-reason":
		record.Attributes().PutStr("aws.vpc.flow.reject_reason", value)
	default:
		return false, nil
	}

	return true, nil
}

// address stores the four fields related to the address
// of a VPC flow log: srcaddr, pkt-srcaddr, dstaddr, and
// pkt-dstaddr. We save these fields in a struct, so we
// can use the right naming conventions in the end.
//
// See https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-records-examples.html#flow-log-example-nat.
type address struct {
	source         string
	pktSource      string
	destination    string
	pktDestination string
}
