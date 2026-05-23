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
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
)

var (
	supportedVPCFlowLogFileFormat = []string{constants.FileFormatPlainText, constants.FileFormatParquet}
	defaultFormat                 = []string{"version", "account-id", "interface-id", "srcaddr", "dstaddr", "srcport", "dstport", "protocol", "packets", "bytes", "start", "end", "action", "log-status"}
)

var _ unmarshaler.StreamingLogsUnmarshaler = (*VPCFlowLogUnmarshaler)(nil)

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

type VPCFlowLogUnmarshaler struct {
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
) (*VPCFlowLogUnmarshaler, error) {
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

	return &VPCFlowLogUnmarshaler{
		cfg:                              cfg,
		buildInfo:                        buildInfo,
		logger:                           logger,
		vpcFlowStartISO8601FormatEnabled: vpcFlowStartISO8601FormatEnabled,
	}, nil
}

func (v *VPCFlowLogUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	switch v.cfg.FileFormat {
	case constants.FileFormatPlainText:
		// Decode as a stream but flush all at once using flush options
		streamUnmarshaler, err := v.NewLogsDecoder(reader, encoding.WithFlushItems(0), encoding.WithFlushBytes(0))
		if err != nil {
			return plog.Logs{}, err
		}
		logs, err := streamUnmarshaler.DecodeLogs()
		if err != nil {
			//nolint:errorlint
			if err == io.EOF {
				// EOF indicates no logs were found, return any logs that's available
				return logs, nil
			}

			return plog.Logs{}, err
		}

		return logs, nil
	case constants.FileFormatParquet:
		// TODO
		return plog.Logs{}, errors.New("still needs to be implemented")
	default:
		// not possible, prevent by NewVPCFlowLogUnmarshaler
		return plog.Logs{}, nil
	}
}

// NewLogsDecoder returns a LogsDecoder that processes VPC flow logs from the provided reader.
// Auto-detects the source format (S3 plain text or CloudWatch subscription filter) from the first byte.
// Supported sub formats:
//   - S3 plain text logs: Supports offset-based streaming; offset tracked by bytes processed
//   - CloudWatch subscription filter: Processes full payload; offset tracked by bytes processed
//   - Parquet format: Not yet implemented
func (v *VPCFlowLogUnmarshaler) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	if v.cfg.FileFormat == constants.FileFormatParquet {
		return nil, errors.New("streaming parquet VPC flow logs is not yet implemented")
	}

	// use buffered reader for efficiency and to avoid any size restrictions
	bufReader := bufio.NewReader(reader)

	var err error
	firstByte, err := bufReader.Peek(1)
	if err != nil {
		return nil, fmt.Errorf("failed to read first byte: %w", err)
	}

	if firstByte[0] == '{' {
		// Dealing with a JSON log message, so check for CloudWatch bound trigger

		decoderOpts := encoding.DecoderOptions{}
		for _, op := range options {
			op(&decoderOpts)
		}

		// If offset is set, return EOF after consuming the whole record.
		// This confirms to our contract - process full payload
		// However, we cannot skip the offset bytes as we need full record unmarshaling.
		if decoderOpts.Offset > 0 {
			var l int64
			l, err = io.Copy(io.Discard, bufReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read the input stream: %w", err)
			}

			return xstreamencoding.NewLogsDecoderAdapter(
				func() (plog.Logs, error) {
					return plog.NewLogs(), io.EOF
				},
				func() int64 {
					return l
				},
			), nil
		}

		var cwLogs plog.Logs
		var offset int64
		cwLogs, offset, err = v.fromCloudWatch(v.cfg.parsedFormat, bufReader)
		if err != nil {
			return nil, err
		}

		var isEOF bool
		return xstreamencoding.NewLogsDecoderAdapter(
			func() (plog.Logs, error) {
				if isEOF {
					return plog.Logs{}, io.EOF
				}

				isEOF = true
				return cwLogs, nil
			},
			func() int64 {
				return offset
			},
		), nil
	}

	var offset int64
	line, err := bufReader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read first line of VPC logs from S3: %w", err)
	}

	offset += int64(len(line))

	fields := strings.Fields(line)
	batchHelper := xstreamencoding.NewBatchHelper(options...)

	if batchHelper.Options().Offset > 0 {
		// discard bytes, ignoring the first line
		var discarded int
		discarded, err = bufReader.Discard(int(batchHelper.Options().Offset - offset))
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("EOF reached before offset %d bytes were discarded", batchHelper.Options().Offset)
			}
			return nil, err
		}
		offset += int64(discarded)
	}

	offsetF := func() int64 {
		return offset
	}

	decodeF := func() (plog.Logs, error) {
		logs, resourceLogs, scopeLogs := v.createLogs()
		for {
			line, err = bufReader.ReadString('\n')
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return plog.Logs{}, fmt.Errorf("error reading VPC logs: %w", err)
				}

				if line == "" {
					break
				}
			}
			batchHelper.IncrementBytes(int64(len(line)))
			batchHelper.IncrementItems(1)
			offset += int64(len(line))

			// Trim spaces and new lines
			line = strings.TrimSpace(line)
			if err := v.addToLogs(resourceLogs, scopeLogs, fields, line); err != nil {
				return plog.Logs{}, err
			}

			if batchHelper.ShouldFlush() {
				batchHelper.Reset()
				break
			}
		}

		if scopeLogs.LogRecords().Len() == 0 {
			return logs, io.EOF
		}

		return logs, nil
	}

	return xstreamencoding.NewLogsDecoderAdapter(decodeF, offsetF), nil
}

// fromCloudWatch expects VPC logs from CloudWatch Logs subscription filter trigger
func (v *VPCFlowLogUnmarshaler) fromCloudWatch(fields []string, reader *bufio.Reader) (plog.Logs, int64, error) {
	var cwLog events.CloudwatchLogsData

	decoder := gojson.NewDecoder(reader)
	err := decoder.Decode(&cwLog)
	if err != nil {
		return plog.Logs{}, 0, fmt.Errorf("failed to unmarshal data as cloudwatch logs event: %w", err)
	}

	logs, resourceLogs, scopeLogs := v.createLogs()

	resourceAttrs := resourceLogs.Resource().Attributes()
	resourceAttrs.PutStr(string(conventions.AWSLogGroupNamesKey), cwLog.LogGroup)
	resourceAttrs.PutStr(string(conventions.AWSLogStreamNamesKey), cwLog.LogStream)

	for _, event := range cwLog.LogEvents {
		err := v.addToLogs(resourceLogs, scopeLogs, fields, event.Message)
		if err != nil {
			return plog.Logs{}, 0, err
		}
	}

	return logs, decoder.InputOffset(), nil
}

// createLogs is a helper to create prefilled plog.Logs, plog.ResourceLogs, plog.ScopeLogs
func (v *VPCFlowLogUnmarshaler) createLogs() (plog.Logs, plog.ResourceLogs, plog.ScopeLogs) {
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
func (v *VPCFlowLogUnmarshaler) addToLogs(
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
func (v *VPCFlowLogUnmarshaler) handleAddresses(addr *address, record plog.LogRecord) {
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
func (v *VPCFlowLogUnmarshaler) handleField(
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
