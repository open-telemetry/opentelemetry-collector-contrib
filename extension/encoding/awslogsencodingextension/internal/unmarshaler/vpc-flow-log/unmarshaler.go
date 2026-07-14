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
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
)

var (
	supportedVPCFlowLogFileFormat = []string{
		constants.FileFormatPlainText,
		constants.FileFormatParquet,
	}

	// defaultVPCFormat holds the default format for VPC flow logs, when
	// sent to CloudWatch Logs. For logs sent to S3, the format is
	// determined from the header line.
	//
	// Note: order is important.
	defaultVPCFormat = []string{
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

	// defaultTGWFormat holds the default format for Transit Gateway flow logs,
	// when sent to CloudWatch Logs. For logs sent to S3, the format is determined
	// from the header line.
	//
	// Note: order is important.
	defaultTGWFormat = []string{
		"version",
		"resource-type",
		"account-id",
		"tgw-id",
		"tgw-attachment-id",
		"tgw-src-vpc-id",
		"tgw-dst-vpc-id",
		"tgw-src-subnet-id",
		"tgw-dst-subnet-id",
		"tgw-src-eni",
		"tgw-dst-eni",
		"tgw-src-az-id",
		"tgw-dst-az-id",
		"srcaddr",
		"dstaddr",
		"srcport",
		"dstport",
		"protocol",
		"packets",
		"bytes",
		"start",
		"end",
		"log-status",
	}
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

	return &VPCFlowLogUnmarshaler{
		cfg:                              cfg,
		buildInfo:                        buildInfo,
		logger:                           logger,
		vpcFlowStartISO8601FormatEnabled: vpcFlowStartISO8601FormatEnabled,
	}, nil
}

func (v *VPCFlowLogUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	streamUnmarshaler, err := v.NewLogsDecoder(reader, encoding.WithFlushItems(0), encoding.WithFlushBytes(0))
	if err != nil {
		return plog.Logs{}, err
	}
	logs, err := streamUnmarshaler.DecodeLogs()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return logs, nil
		}
		return plog.Logs{}, err
	}
	return logs, nil
}

// NewLogsDecoder returns a LogsDecoder that processes VPC flow logs from the provided reader.
// For plain text, auto-detects the source (S3 or CloudWatch subscription filter) from the first byte.
// Supported sub formats:
//   - S3 plain text logs: Supports offset-based streaming; offset tracked by bytes processed
//   - CloudWatch subscription filter: Processes full payload; offset tracked by bytes processed
//   - Parquet format: Batches rows according to flush options; offset tracked by rows processed.
//     If the reader implements io.ReaderAt (with Size() or io.Seeker), the file is opened
//     directly without buffering into memory. WithOffset skips rows via SeekToRow.
func (v *VPCFlowLogUnmarshaler) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	if v.cfg.FileFormat == constants.FileFormatParquet {
		return v.newParquetLogsDecoder(reader, options...)
	}
	return v.newPlaintextLogsDecoder(reader, options...)
}

func (v *VPCFlowLogUnmarshaler) newPlaintextLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
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

	if fields == nil {
		// No format specified, so we assume the default format. The default format is different
		// for Transit Gateway and plain VPC flow logs, so we need to inspect the log message.
		if len(cwLog.LogEvents) > 0 {
			// The 2nd field is "TransitGateway" for TGW logs (resource-type field)
			_, rest, ok := strings.Cut(cwLog.LogEvents[0].Message, " ")
			if ok && strings.HasPrefix(rest, "TransitGateway ") {
				fields = defaultTGWFormat
				v.logger.Debug("Detected TGW flow log format for CloudWatch stream")
			} else {
				fields = defaultVPCFormat
			}
			v.cfg.parsedFormat = fields
		}
	}

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

// openParquetFile opens a parquet.File from the given reader. To avoid buffering
// the entire file into memory, the reader may implement io.ReaderAt with either
// a Size() int64 method or io.Seeker. If neither is available, the full payload
// is read into a buffer first.
func openParquetFile(reader io.Reader) (*parquet.File, error) {
	// readerAtSizer is for readers that implement io.ReaderAt and have a
	// Size() method, such as bytes.Reader.
	type readerAtSizer interface {
		io.ReaderAt
		Size() int64
	}
	// readerAtSeeker is for readers that implement io.ReaderAt and io.Seeker,
	// such as os.File. We can determine the size by seeking to the end, then
	// reset to the start for reading.
	type readerAtSeeker interface {
		io.ReaderAt
		io.Seeker
	}

	var readerAt io.ReaderAt
	var size int64
	switch r := reader.(type) {
	case readerAtSizer:
		readerAt = r
		size = r.Size()
	case readerAtSeeker:
		var err error
		size, err = r.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, fmt.Errorf("failed to seek parquet reader: %w", err)
		}
		if _, err = r.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to reset parquet reader: %w", err)
		}
		readerAt = r
	default:
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read parquet data: %w", err)
		}
		readerAt = bytes.NewReader(data)
		size = int64(len(data))
	}
	return parquet.OpenFile(readerAt, size)
}

// newParquetLogsDecoder returns a LogsDecoder that yields batches of Parquet rows
// according to the flush options. Offset is measured in rows: WithOffset skips
// rows efficiently using SeekToRow, and Offset() returns the total rows consumed.
//
// To avoid buffering the entire file into memory, the reader may implement
// io.ReaderAt with either a Size() int64 method or io.Seeker. If neither is
// available, the full payload is read into a buffer first.
func (v *VPCFlowLogUnmarshaler) newParquetLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	file, err := openParquetFile(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file: %w", err)
	}

	columnPaths := file.Schema().Columns()
	fields := make([]string, len(columnPaths))
	for i, columnPath := range columnPaths {
		// Convert underscores to hyphens to match the JSON encoding field names.
		fields[i] = strings.ReplaceAll(strings.Join(columnPath, "."), "_", "-")
	}
	v.warnUnknownParquetColumns(fields)

	rowGroups := file.RowGroups()
	batchHelper := xstreamencoding.NewBatchHelper(options...)

	// For Parquet, offset is measured in rows (not bytes).
	rowOffset := batchHelper.Options().Offset

	// State for iterating across row groups.
	groupIdx := 0
	var rowReader parquet.Rows
	var rowBuf [1]parquet.Row

	// Skip whole row groups and seek within the target group using SeekToRow.
	if rowOffset > 0 {
		remaining := rowOffset
		for groupIdx < len(rowGroups) {
			n := rowGroups[groupIdx].NumRows()
			if remaining < n {
				// Partial skip within this row group.
				rowReader = rowGroups[groupIdx].Rows()
				if err = rowReader.SeekToRow(remaining); err != nil {
					_ = rowReader.Close()
					return nil, fmt.Errorf("failed to seek to row %d in parquet row group: %w", remaining, err)
				}
				break
			}
			remaining -= n
			groupIdx++
		}
	}

	decodeF := func() (plog.Logs, error) {
		logs, resourceLogs, scopeLogs := v.createLogs()
		for groupIdx < len(rowGroups) {
			if rowReader == nil {
				rowReader = rowGroups[groupIdx].Rows()
			}
			for {
				n, readErr := rowReader.ReadRows(rowBuf[:])
				if n > 0 {
					if err := v.addParquetRowToLogs(rowBuf[0], fields, resourceLogs, scopeLogs); err != nil {
						_ = rowReader.Close()
						return plog.Logs{}, err
					}
					rowOffset++
					batchHelper.IncrementItems(1)
				}
				if readErr != nil {
					if err := rowReader.Close(); err != nil {
						return plog.Logs{}, fmt.Errorf("failed to close parquet row reader: %w", err)
					}
					rowReader = nil
					groupIdx++
					if !errors.Is(readErr, io.EOF) {
						return plog.Logs{}, fmt.Errorf("failed to read parquet rows: %w", readErr)
					}
					break
				}
				if batchHelper.ShouldFlush() {
					batchHelper.Reset()
					return logs, nil
				}
			}
		}
		if scopeLogs.LogRecords().Len() == 0 {
			return logs, io.EOF
		}
		return logs, nil
	}

	return xstreamencoding.NewLogsDecoderAdapter(decodeF, func() int64 { return rowOffset }), nil
}

// warnUnknownParquetColumns logs a warning for each column name that is not
// recognized by either handleStringField or handleInt64Field. Invoked once at
// decoder construction since the Parquet schema is fixed for the file.
func (v *VPCFlowLogUnmarshaler) warnUnknownParquetColumns(fields []string) {
	scratchLogs := plog.NewLogs()
	scratchResLogs := scratchLogs.ResourceLogs().AppendEmpty()
	scratchRecord := scratchResLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	var scratchAddr address
	for _, field := range fields {
		foundStr, _ := v.handleStringField(field, "", scratchResLogs, scratchRecord, &scratchAddr)
		foundInt, _ := v.handleInt64Field(field, 0, scratchResLogs, scratchRecord, &scratchAddr)
		if !foundStr && !foundInt {
			v.logger.Warn("field is not an available field for a flow log record",
				zap.String("field", field),
				zap.String("documentation", "https://docs.aws.amazon.com/vpc/latest/userguide/flow-log-records.html"),
			)
		}
	}
}

// addParquetRowToLogs processes a single Parquet row and appends a log record.
func (v *VPCFlowLogUnmarshaler) addParquetRowToLogs(
	row parquet.Row,
	fields []string,
	resourceLogs plog.ResourceLogs,
	scopeLogs plog.ScopeLogs,
) error {
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	var addr address
	var columnErr error
	row.Range(func(columnIndex int, columnValues []parquet.Value) bool {
		if len(columnValues) == 0 {
			return true
		}
		val := columnValues[0]
		if val.IsNull() {
			return true
		}
		if columnIndex >= len(fields) {
			return true
		}

		field := fields[columnIndex]
		switch val.Kind() {
		case parquet.Int32:
			_, columnErr = v.handleInt64Field(field, int64(val.Int32()), resourceLogs, logRecord, &addr)
		case parquet.Int64:
			_, columnErr = v.handleInt64Field(field, val.Int64(), resourceLogs, logRecord, &addr)
		default:
			_, columnErr = v.handleStringField(field, val.String(), resourceLogs, logRecord, &addr)
		}
		return columnErr == nil
	})
	if columnErr != nil {
		return columnErr
	}
	v.handleAddresses(&addr, logRecord)
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
	if value == "-" {
		return true, nil
	}
	// Integer fields: parse and call handleInt64Field
	switch field {
	case "version", "srcport", "dstport", "protocol", "packets", "bytes",
		"tcp-flags", "traffic-path", "start", "end",
		"packets-lost-no-route", "packets-lost-blackhole",
		"packets-lost-mtu-exceeded", "packets-lost-ttl-expired":
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
func (*VPCFlowLogUnmarshaler) handleStringField(
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
	case "tgw-id":
		record.Attributes().PutStr("aws.tgw.id", value)
	case "tgw-attachment-id":
		record.Attributes().PutStr("aws.tgw.attachment.id", value)
	case "tgw-src-vpc-id":
		record.Attributes().PutStr("aws.tgw.source.vpc.id", value)
	case "tgw-dst-vpc-id":
		record.Attributes().PutStr("aws.tgw.destination.vpc.id", value)
	case "tgw-src-subnet-id":
		record.Attributes().PutStr("aws.tgw.source.vpc.subnet.id", value)
	case "tgw-dst-subnet-id":
		record.Attributes().PutStr("aws.tgw.destination.vpc.subnet.id", value)
	case "tgw-src-eni":
		record.Attributes().PutStr("aws.tgw.source.eni.id", value)
	case "tgw-dst-eni":
		record.Attributes().PutStr("aws.tgw.destination.eni.id", value)
	case "tgw-src-az-id":
		record.Attributes().PutStr("aws.tgw.source.az.id", value)
	case "tgw-dst-az-id":
		record.Attributes().PutStr("aws.tgw.destination.az.id", value)
	case "tgw-pair-attachment-id":
		record.Attributes().PutStr("aws.tgw.attachment.pair.id", value)
	case "resource-type":
		// Skip - used for detection only, already captured in encoding.format
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
func (v *VPCFlowLogUnmarshaler) handleInt64Field(
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
			// New behavior: ISO-8601 format (RFC3339Nano)
			timestamp := time.Unix(value, 0).UTC()
			record.Attributes().PutStr("aws.vpc.flow.start", timestamp.Format(time.RFC3339Nano))
		} else {
			// Legacy behavior: Unix timestamp as integer
			record.Attributes().PutInt("aws.vpc.flow.start", value)
		}
	case "end":
		record.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(value, 0)))
	case "tcp-flags":
		record.Attributes().PutStr("network.tcp.flags", strconv.FormatInt(value, 10))
	case "traffic-path":
		record.Attributes().PutStr("aws.vpc.flow.traffic_path", strconv.FormatInt(value, 10))
	case "packets-lost-no-route":
		record.Attributes().PutInt("aws.vpc.flow.packets_lost_no_route", value)
	case "packets-lost-blackhole":
		record.Attributes().PutInt("aws.vpc.flow.packets_lost_blackhole", value)
	case "packets-lost-mtu-exceeded":
		record.Attributes().PutInt("aws.vpc.flow.packets_lost_mtu_exceeded", value)
	case "packets-lost-ttl-expired":
		record.Attributes().PutInt("aws.vpc.flow.packets_lost_ttl_expired", value)
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
