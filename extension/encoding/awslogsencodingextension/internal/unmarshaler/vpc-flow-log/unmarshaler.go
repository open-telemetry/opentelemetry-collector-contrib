// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	gojson "github.com/goccy/go-json"
	"github.com/parquet-go/parquet-go"
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
	supportedVPCFlowLogFileFormat = []string{
		constants.FileFormatPlainText,
		constants.FileFormatParquet,
	}

	// Note: order is important.
	defaultFormat = []string{
		"version",
		"account-id",
		"interface-id",
		"srcaddr",
		"dstaddr",
		"srcport",
		"dstport",
		"protocol",
		"packets",
		"bytes",
		"start",
		"end",
		"action",
		"log-status",
	}
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
	case constants.FileFormatParquet: // valid
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
		return v.unmarshalParquetLogs(reader)
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

// addToLogs parses the space-separated log line using the given field order and sets plog attributes directly.
// Too few or too many fields returns an error.
func (v *vpcFlowLogUnmarshaler) addToLogs(
	resourceLogs plog.ResourceLogs,
	scopeLogs plog.ScopeLogs,
	fields []string,
	logLine string,
) error {
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	var addr address
	for _, field := range fields {
		if logLine == "" {
			return errors.New("log line has less fields than the ones expected")
		}
		var value string
		value, logLine, _ = strings.Cut(logLine, " ")
		found, err := v.handleField(field, value, resourceLogs, logRecord, &addr)
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
	v.handleAddresses(&addr, logRecord)
	return nil
}

func (v *vpcFlowLogUnmarshaler) unmarshalParquetLogs(reader io.Reader) (plog.Logs, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to read parquet data: %w", err)
	}
	r := bytes.NewReader(data)
	size := int64(len(data))

	file, err := parquet.OpenFile(r, size)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to open parquet file: %w", err)
	}

	columnPaths := file.Schema().Columns()
	fields := make([]string, len(columnPaths))
	for i, columnPath := range columnPaths {
		// Convert underscores to hyphens to match the JSON encoding field names.
		fields[i] = strings.ReplaceAll(strings.Join(columnPath, "."), "_", "-")
	}

	logs, resourceLogs, scopeLogs := v.createLogs()
	var rowBuf [10]parquet.Row
	for _, rowGroup := range file.RowGroups() {
		rowReader := rowGroup.Rows()
		for {
			n, err := rowReader.ReadRows(rowBuf[:])
			for _, row := range rowBuf[:n] {
				logRecord := scopeLogs.LogRecords().AppendEmpty()
				var addr address
				var columnErr error // set below if any column processing fails
				row.Range(func(columnIndex int, columnValues []parquet.Value) bool {
					if len(columnValues) == 0 {
						return true
					}
					val := columnValues[0]
					if val.IsNull() {
						return true
					}
					if columnIndex >= len(columnPaths) {
						return true
					}

					field := fields[columnIndex]
					switch val.Kind() {
					case parquet.Int32:
						_, err = v.handleInt64Field(field, int64(val.Int32()), resourceLogs, logRecord, &addr)
					case parquet.Int64:
						_, err = v.handleInt64Field(field, val.Int64(), resourceLogs, logRecord, &addr)
					default:
						_, err = v.handleStringField(field, val.String(), resourceLogs, logRecord, &addr)
					}
					return columnErr == nil
				})
				if columnErr != nil {
					err = columnErr
					break
				}
				v.handleAddresses(&addr, logRecord)
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				_ = rowReader.Close()
				return plog.Logs{}, fmt.Errorf("failed to read parquet rows: %w", err)
			}
		}
		if err := rowReader.Close(); err != nil {
			return plog.Logs{}, fmt.Errorf("failed to close parquet row reader: %w", err)
		}
	}
	return logs, nil
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

// handleField parses the string value and delegates to handleStringField or handleInt64Field.
// If the field is not recognized, it returns false.
func (v *vpcFlowLogUnmarshaler) handleField(
	field string,
	value string,
	resourceLogs plog.ResourceLogs,
	record plog.LogRecord,
	addr *address,
) (bool, error) {
	if value == "-" {
		return true, nil
	}
	// Integer fields: parse and call handleInt64Field
	switch field {
	case "version", "srcport", "dstport", "protocol", "packets", "bytes", "tcp-flags", "traffic-path", "start", "end":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return false, fmt.Errorf("%q field in log file is not a number", field)
		}
		return v.handleInt64Field(field, n, resourceLogs, record, addr)
	}
	return v.handleStringField(field, value, resourceLogs, record, addr)
}

// handleStringField applies a string value for the given field to resource/record/addr.
// If the field is not recognized, it returns false.
func (*vpcFlowLogUnmarshaler) handleStringField(
	field string,
	value string,
	resourceLogs plog.ResourceLogs,
	record plog.LogRecord,
	addr *address,
) (bool, error) {
	if value == "-" {
		return true, nil
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
		record.Attributes().PutStr(string(conventions.NetworkInterfaceNameKey), value)
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

// handleInt64Field applies an int64 value for the given field to resource/record.
// Returns (true, nil) when the field is recognized, (false, nil) when unknown, or (_, err) on error.
func (v *vpcFlowLogUnmarshaler) handleInt64Field(
	field string,
	value int64,
	_ plog.ResourceLogs,
	record plog.LogRecord,
	_ *address,
) (bool, error) {
	switch field {
	case "version":
		record.Attributes().PutInt("aws.vpc.flow.log.version", value)
	case "srcport":
		record.Attributes().PutInt(string(conventions.SourcePortKey), value)
	case "dstport":
		record.Attributes().PutInt(string(conventions.DestinationPortKey), value)
	case "protocol":
		protocolNumber := int(value)
		if protocolNumber < 0 || protocolNumber >= len(protocolNames) {
			return false, fmt.Errorf("protocol number %d does not have a protocol name", protocolNumber)
		}
		record.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), protocolNames[protocolNumber])
	case "packets":
		record.Attributes().PutInt("aws.vpc.flow.packets", value)
	case "bytes":
		record.Attributes().PutInt("aws.vpc.flow.bytes", value)
	case "start":
		if v.vpcFlowStartISO8601FormatEnabled {
			timestamp := time.Unix(value, 0).UTC()
			record.Attributes().PutStr("aws.vpc.flow.start", timestamp.Format(time.RFC3339Nano))
		} else {
			record.Attributes().PutInt("aws.vpc.flow.start", value)
		}
	case "end":
		record.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(value, 0)))
	case "tcp-flags":
		record.Attributes().PutStr("network.tcp.flags", strconv.FormatInt(value, 10))
	case "traffic-path":
		record.Attributes().PutStr("aws.vpc.flow.traffic_path", strconv.FormatInt(value, 10))
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
