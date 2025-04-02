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
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
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

	scopeLogsByResource := map[resourceKey]plog.ScopeLogs{}
	for scanner.Scan() {
		line := scanner.Text()
		if err := v.addToLogs(scopeLogsByResource, fields, line); err != nil {
			return plog.Logs{}, err
		}
	}

	if err := scanner.Err(); err != nil {
		return plog.Logs{}, fmt.Errorf("error reading log line: %w", err)
	}

	return v.createLogs(scopeLogsByResource), nil
}

// createLogs based on the scopeLogsByResource map
func (v *vpcFlowLogUnmarshaler) createLogs(scopeLogsByResource map[resourceKey]plog.ScopeLogs) plog.Logs {
	logs := plog.NewLogs()

	for key, scopeLogs := range scopeLogsByResource {
		rl := logs.ResourceLogs().AppendEmpty()
		attr := rl.Resource().Attributes()
		attr.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
		if key.accountID != "" {
			attr.PutStr(conventions.AttributeCloudAccountID, key.accountID)
		}
		if key.region != "" {
			attr.PutStr(conventions.AttributeCloudRegion, key.region)
		}
		scopeLogs.MoveTo(rl.ScopeLogs().AppendEmpty())
	}

	return logs
}

// addToLogs parses the log line and creates
// a new record log. The record log is added
// to the scope logs of the resource identified
// by the resourceKey created from the values.
func (v *vpcFlowLogUnmarshaler) addToLogs(
	scopeLogsByResource map[resourceKey]plog.ScopeLogs,
	fields []string,
	logLine string,
) error {
	// first line includes the fields
	// TODO Replace with an iterator starting from go 1.24:
	// https://pkg.go.dev/strings#FieldsSeq
	nFields := len(fields)
	nValues := strings.Count(logLine, " ") + 1
	if nFields != nValues {
		return fmt.Errorf("expect %d fields per log line, got log line with %d fields", nFields, nValues)
	}

	// create new key for resource and new
	// log record to add to the scope of logs
	// of the resource
	key := &resourceKey{}
	record := plog.NewLogRecord()

	// There are 4 fields for the addresses: srcaddr, pkt-srcaddr,
	// dstaddr, pkt-dstaddr. We will save these fields in a
	// map, so we can use the right conventions in the end.
	//
	// See https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-records-examples.html#flow-log-example-nat.
	ips := make(map[string]string, 4)

	start := 0
	for _, field := range fields {
		var value string
		end := strings.Index(logLine[start:], " ")
		if end == -1 {
			value = logLine[start:]
		} else {
			value = logLine[start : start+end]
			start += end + 1 // skip the space
		}

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

		found, err := handleField(field, value, record, ips, key)
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

	// get the address fields
	addresses := v.handleAddresses(ips)
	for field, value := range addresses {
		record.Attributes().PutStr(field, value)
	}

	scopeLogs := v.getScopeLogs(*key, scopeLogsByResource)
	rScope := scopeLogs.LogRecords().AppendEmpty()
	record.MoveTo(rScope)

	return nil
}

// handleAddresses creates a new map where the original field
// names will be the known conventions for the fields
func (v *vpcFlowLogUnmarshaler) handleAddresses(addresses map[string]string) map[string]string {
	// max is 3 fields, see example in
	// https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-records-examples.html#flow-log-example-nat
	recordAttr := make(map[string]string, 3)
	srcaddr, foundSrc := addresses["srcaddr"]
	pktSrcaddr, foundSrcPkt := addresses["pkt-srcaddr"]
	if !foundSrcPkt && foundSrc {
		// there is no middle layer, assume "srcaddr" field
		// corresponds to the original source address.
		recordAttr[conventions.AttributeSourceAddress] = srcaddr
	} else if foundSrcPkt && foundSrc {
		recordAttr[conventions.AttributeSourceAddress] = pktSrcaddr
		if srcaddr != pktSrcaddr {
			// srcaddr is the middle layer
			recordAttr[conventions.AttributeNetworkPeerAddress] = srcaddr
		}
	}

	dstaddr, foundDst := addresses["dstaddr"]
	pktDstaddr, foundDstPkt := addresses["pkt-dstaddr"]
	if !foundDstPkt && foundDst {
		// there is no middle layer, assume "dstaddr" field
		// corresponds to the original destination address.
		recordAttr[conventions.AttributeDestinationAddress] = dstaddr
	} else if foundDstPkt && foundDst {
		recordAttr[conventions.AttributeDestinationAddress] = pktDstaddr
		if pktDstaddr != dstaddr {
			if _, found := recordAttr[conventions.AttributeNetworkPeerAddress]; found {
				v.logger.Warn("unexpected: srcaddr, dstaddr, pkt-srcaddr and pkt-dstaddr are all different")
			}
			// dstaddr is the middle layer
			recordAttr[conventions.AttributeNetworkPeerAddress] = dstaddr
		}
	}

	return recordAttr
}

// getScopeLogs for the given key. If it does not exist yet,
// create new scope logs, and add the key to the logs map.
func (v *vpcFlowLogUnmarshaler) getScopeLogs(key resourceKey, logs map[resourceKey]plog.ScopeLogs) plog.ScopeLogs {
	scopeLogs, ok := logs[key]
	if !ok {
		scopeLogs = plog.NewScopeLogs()
		scopeLogs.Scope().SetName(metadata.ScopeName)
		scopeLogs.Scope().SetVersion(v.buildInfo.Version)
		logs[key] = scopeLogs
	}
	return scopeLogs
}

// handleField analyzes the given field and it either
// adds its value to the resourceKey or puts the
// field and its value in the attributes map. If the
// field is not recognized, it returns false.
func handleField(
	field string,
	value string,
	record plog.LogRecord,
	ips map[string]string,
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
	// TODO Add support for ECS fields
	case "srcaddr", "pkt-srcaddr", "dstaddr", "pkt-dstaddr":
		// handled later
		ips[field] = value
	case "account-id":
		key.accountID = value
	case "vpc-id":
		record.Attributes().PutStr("aws.vpc.id", value)
	case "subnet-id":
		record.Attributes().PutStr("aws.vpc.subnet.id", value)
	case "instance-id":
		record.Attributes().PutStr(conventions.AttributeHostID, value)
	case "az-id":
		record.Attributes().PutStr("aws.az.id", value)
	case "interface-id":
		record.Attributes().PutStr("aws.eni.id", value)
	case "srcport":
		if err := addNumber(field, value, conventions.AttributeSourcePort); err != nil {
			return false, err
		}
	case "dstport":
		if err := addNumber(field, value, conventions.AttributeDestinationPort); err != nil {
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
		record.Attributes().PutStr(conventions.AttributeNetworkProtocolName, protocolNames[protocolNumber])
	case "type":
		record.Attributes().PutStr(conventions.AttributeNetworkType, strings.ToLower(value))
	case "region":
		key.region = value
	case "flow-direction":
		switch value {
		case "ingress":
			record.Attributes().PutStr(conventions.AttributeNetworkIoDirection, "receive")
		case "egress":
			record.Attributes().PutStr(conventions.AttributeNetworkIoDirection, "transmit")
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
	case "reject-reason":
		record.Attributes().PutStr("aws.vpc.flow.reject_reason", value)
	default:
		return false, nil
	}

	return true, nil
}
