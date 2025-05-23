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
	"sync"
	"time"

	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
)

const (
	fileFormatPlainText = "plain-text"
	fileFormatParquet   = "parquet"
)

var supportedVPCFlowLogFileFormat = []string{fileFormatPlainText, fileFormatParquet}

type vpcFlowLogUnmarshaler struct {
	// VPC flow logs can be sent in plain text
	// or parquet files to S3.
	//
	// See https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-s3-path.html.
	fileFormat string

	// Pool the gzip readers, which are expensive to create.
	gzipPool sync.Pool

	buildInfo component.BuildInfo
	logger    *zap.Logger
}

func NewVPCFlowLogUnmarshaler(format string, buildInfo component.BuildInfo, logger *zap.Logger) (plog.Unmarshaler, error) {
	switch format {
	case fileFormatParquet:
		// TODO
		return nil, errors.New("still needs to be implemented")
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
		logger:     logger,
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
		// TODO
		return plog.Logs{}, errors.New("still needs to be implemented")
	default:
		// not possible, prevent by NewVPCFlowLogUnmarshaler
		return plog.Logs{}, nil
	}
}

// resourceKey stores the account id and region
// of the flow logs. All log lines inside the
// same S3 file come from the same account and
// region.
//
// See https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-s3-path.html.
type resourceKey struct {
	accountID string
	region    string
}

func (v *vpcFlowLogUnmarshaler) unmarshalPlainTextLogs(reader io.Reader) (plog.Logs, error) {
	scanner := bufio.NewScanner(reader)

	var fields []string
	if scanner.Scan() {
		firstLine := scanner.Text()
		fields = strings.Split(firstLine, " ")
	}

	logs, resourceLogs, scopeLogs := v.createLogs()
	key := &resourceKey{}
	for scanner.Scan() {
		line := scanner.Text()
		if err := v.addToLogs(key, scopeLogs, fields, line); err != nil {
			return plog.Logs{}, err
		}
	}

	if err := scanner.Err(); err != nil {
		return plog.Logs{}, fmt.Errorf("error reading log line: %w", err)
	}

	v.setResourceAttributes(key, resourceLogs)
	return logs, nil
}

// createLogs with the expected fields for the scope logs
func (v *vpcFlowLogUnmarshaler) createLogs() (plog.Logs, plog.ResourceLogs, plog.ScopeLogs) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(v.buildInfo.Version)
	return logs, resourceLogs, scopeLogs
}

// setResourceAttributes based on the resourceKey
func (v *vpcFlowLogUnmarshaler) setResourceAttributes(key *resourceKey, logs plog.ResourceLogs) {
	attr := logs.Resource().Attributes()
	attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	if key.accountID != "" {
		attr.PutStr(string(conventions.CloudAccountIDKey), key.accountID)
	}
	if key.region != "" {
		attr.PutStr(string(conventions.CloudRegionKey), key.region)
	}
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

// addToLogs parses the log line and creates
// a new record log. The record log is added
// to the scope logs of the resource identified
// by the resourceKey created from the values.
func (v *vpcFlowLogUnmarshaler) addToLogs(
	key *resourceKey,
	scopeLogs plog.ScopeLogs,
	fields []string,
	logLine string,
) error {
	record := plog.NewLogRecord()

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

		found, err := handleField(field, value, record, addr, key)
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

	// add the address fields with the correct conventions
	// to the log record
	v.handleAddresses(addr, record)
	rScope := scopeLogs.LogRecords().AppendEmpty()
	record.MoveTo(rScope)

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
func handleField(
	field string,
	value string,
	record plog.LogRecord,
	addr *address,
	key *resourceKey,
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
		key.accountID = value
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
		key.region = value
	case "flow-direction":
		switch value {
		case "ingress":
			record.Attributes().PutStr(string(conventions.NetworkIoDirectionKey), "receive")
		case "egress":
			record.Attributes().PutStr(string(conventions.NetworkIoDirectionKey), "transmit")
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
		if err := addNumber(field, value, "aws.vpc.flow.start"); err != nil {
			return false, err
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
