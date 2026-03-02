// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/elb-access-log"

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
)

type elbAccessLogUnmarshaler struct {
	buildInfo component.BuildInfo
	logger    *zap.Logger
}

func NewELBAccessLogUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger) unmarshaler.AWSUnmarshaler {
	return &elbAccessLogUnmarshaler{
		buildInfo: buildInfo,
		logger:    logger,
	}
}

type resourceAttributes struct {
	resourceID string
}

// UnmarshalAWSLogs processes a file containing ELB access logs.
func (f *elbAccessLogUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	scanner := bufio.NewScanner(reader)

	logs, resourceLogs, scopeLogs := f.createLogs()
	resourceAttr := &resourceAttributes{}

	var line string
	var fields []string

	// Read first line to determine format
	if !scanner.Scan() {
		return plog.Logs{}, errors.New("no log lines found")
	}
	line = scanner.Text()

	fields, err := extractFields(line)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to parse log line: %w", err)
	}
	if len(fields) == 0 {
		return plog.Logs{}, fmt.Errorf("log line has no fields: %s", line)
	}

	// Check for control message
	if fields[0] == EnableControlMessage {
		f.logger.Info(fmt.Sprintf("Control message received: %s", line))
		return plog.NewLogs(), nil
	}

	// Determine syntax
	syntax, err := findLogSyntaxByField(fields[0])
	if err != nil {
		return plog.Logs{}, fmt.Errorf("unable to determine log syntax: %w", err)
	}
	for {
		// Process lines based on determined syntax
		switch syntax {
		case albAccessLogs:
			err = f.handleALBAccessLogs(fields, resourceAttr, scopeLogs)
			if err != nil {
				return plog.Logs{}, err
			}
		case nlbAccessLogs:
			err = f.handleNLBAccessLogs(fields, resourceAttr, scopeLogs)
			if err != nil {
				return plog.Logs{}, err
			}
		case clbAccessLogs:
			err = f.handleCLBAccessLogs(fields, resourceAttr, scopeLogs)
			if err != nil {
				return plog.Logs{}, err
			}
		default:
			return plog.Logs{}, fmt.Errorf("unsupported log syntax: %s", syntax)
		}

		// Refill with next line until we reach the scanner end
		if !scanner.Scan() {
			break
		}

		line = scanner.Text()
		fields, err = extractFields(line)
		if err != nil {
			return plog.Logs{}, fmt.Errorf("failed to parse log line: %w", err)
		}
		if len(fields) == 0 {
			return plog.Logs{}, fmt.Errorf("log line has no fields: %s", line)
		}
	}

	// Handle potential scanner errors
	if err := scanner.Err(); err != nil {
		return plog.Logs{}, fmt.Errorf("error scanning log lines: %w", err)
	}

	f.setResourceAttributes(resourceAttr, resourceLogs)
	return logs, nil
}

// createLogs with the expected fields for the scope logs
func (f *elbAccessLogUnmarshaler) createLogs() (plog.Logs, plog.ResourceLogs, plog.ScopeLogs) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(f.buildInfo.Version)
	scopeLogs.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatELBAccessLog)
	return logs, resourceLogs, scopeLogs
}

// setResourceAttributes based on the resourceAttributes
func (*elbAccessLogUnmarshaler) setResourceAttributes(r *resourceAttributes, logs plog.ResourceLogs) {
	attr := logs.Resource().Attributes()
	attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attr.PutStr(string(conventions.CloudResourceIDKey), r.resourceID)
}

// handleCLBAccessLogs handles clb access logs
func (f *elbAccessLogUnmarshaler) handleCLBAccessLogs(fields []string, resourceAttr *resourceAttributes, scopeLogs plog.ScopeLogs) error {
	record, err := convertTextToCLBAccessLogRecord(fields)
	if err != nil {
		return fmt.Errorf("unable to convert log line to CLB record: %w", err)
	}
	f.addToCLBAccessLogs(resourceAttr, scopeLogs, record)
	return nil
}

// addToCLBAccessLogs adds clb record to provided logs based
// on the extracted logs of each resource
func (f *elbAccessLogUnmarshaler) addToCLBAccessLogs(resourceAttr *resourceAttributes, scopeLogs plog.ScopeLogs, clbRecord CLBAccessLogRecord) {
	// Convert timestamp first; if invalid, skip log creation
	epochNanoseconds, err := convertToUnixEpoch(clbRecord.Time)
	if err != nil {
		f.logger.Debug("Timestamp cannot be converted to unix epoch nanoseconds", zap.Error(err))
		return
	}

	// Create record log
	recordLog := plog.NewLogRecord()
	// Set resource id
	resourceAttr.resourceID = clbRecord.ELB
	// Populate record attributes
	recordLog.Attributes().PutStr(string(conventions.ClientAddressKey), clbRecord.ClientIP)
	recordLog.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), clbRecord.RequestMethod)
	recordLog.Attributes().PutStr(string(conventions.URLFullKey), clbRecord.RequestURI)
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), clbRecord.ProtocolName)
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolVersionKey), clbRecord.ProtocolVersion)
	recordLog.Attributes().PutInt(string(conventions.ClientPortKey), clbRecord.ClientPort)
	recordLog.Attributes().PutInt(string(conventions.HTTPRequestSizeKey), clbRecord.ReceivedBytes)
	recordLog.Attributes().PutInt(string(conventions.HTTPResponseSizeKey), clbRecord.SentBytes)
	if clbRecord.SSLProtocol != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSProtocolVersionKey), strings.ToLower(clbRecord.SSLProtocol))
	}
	if clbRecord.SSLCipher != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSCipherKey), clbRecord.SSLCipher)
	}
	if clbRecord.ELBStatusCode != 0 {
		recordLog.Attributes().PutInt(AttributeELBStatusCode, clbRecord.ELBStatusCode)
	}
	if clbRecord.BackendStatusCode != 0 {
		recordLog.Attributes().PutInt(AttributeELBBackendStatusCode, clbRecord.BackendStatusCode)
	}

	if clbRecord.UserAgent != unknownField {
		recordLog.Attributes().PutStr(string(conventions.UserAgentOriginalKey), clbRecord.UserAgent)
	}
	if clbRecord.BackendIPPort != unknownField {
		recordLog.Attributes().PutStr(string(conventions.DestinationAddressKey), clbRecord.BackendIP)
		recordLog.Attributes().PutInt(string(conventions.DestinationPortKey), clbRecord.BackendPort)
	}
	// Set timestamp
	recordLog.SetTimestamp(pcommon.Timestamp(epochNanoseconds))

	// move recordLog to scope
	rScope := scopeLogs.LogRecords().AppendEmpty()
	recordLog.MoveTo(rScope)
}

// handleALBAccessLogs handles alb access logs
func (f *elbAccessLogUnmarshaler) handleALBAccessLogs(fields []string, resourceAttr *resourceAttributes, scopeLogs plog.ScopeLogs) error {
	record, err := convertTextToALBAccessLogRecord(fields)
	if err != nil {
		return fmt.Errorf("unable to convert log line to ALB record: %w", err)
	}
	f.addToALBAccessLogs(resourceAttr, scopeLogs, record)
	return nil
}

// addToALBAccessLogs adds alb record to provided logs based
// on the extracted logs of each resource
func (f *elbAccessLogUnmarshaler) addToALBAccessLogs(resourceAttr *resourceAttributes, scopeLogs plog.ScopeLogs, albRecord ALBAccessLogRecord) {
	// Convert timestamp first; if invalid, skip log creation
	epochNanoseconds, err := convertToUnixEpoch(albRecord.Time)
	if err != nil {
		f.logger.Debug("Timestamp cannot be converted to unix epoch nanoseconds", zap.Error(err))
		return
	}

	// Create record log
	recordLog := plog.NewLogRecord()
	// Set resource id
	resourceAttr.resourceID = albRecord.ELB
	// Populate record attributes
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), albRecord.Type)
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolVersionKey), albRecord.ProtocolVersion)
	recordLog.Attributes().PutStr(string(conventions.ClientAddressKey), albRecord.ClientIP)
	recordLog.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), albRecord.RequestMethod)
	recordLog.Attributes().PutStr(string(conventions.URLFullKey), albRecord.RequestURI)
	recordLog.Attributes().PutInt(string(conventions.ClientPortKey), albRecord.ClientPort)
	recordLog.Attributes().PutInt(string(conventions.HTTPRequestSizeKey), albRecord.ReceivedBytes)
	recordLog.Attributes().PutInt(string(conventions.HTTPResponseSizeKey), albRecord.SentBytes)
	recordLog.Attributes().PutInt(AttributeELBStatusCode, albRecord.ELBStatusCode)
	if albRecord.SSLProtocol != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSProtocolVersionKey), strings.ToLower(albRecord.SSLProtocol))
	}
	if albRecord.SSLCipher != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSCipherKey), albRecord.SSLCipher)
	}
	if albRecord.UserAgent != unknownField {
		recordLog.Attributes().PutStr(string(conventions.UserAgentOriginalKey), albRecord.UserAgent)
	}
	if albRecord.DomainName != unknownField {
		recordLog.Attributes().PutStr(string(conventions.URLDomainKey), albRecord.DomainName)
	}
	if albRecord.TargetIPPort != unknownField {
		recordLog.Attributes().PutStr(string(conventions.DestinationAddressKey), albRecord.TargetIP)
		recordLog.Attributes().PutInt(string(conventions.DestinationPortKey), albRecord.TargetPort)
	}

	// Times are expressed in seconds with a precision of 3 decimal places in logs. Here we convert them to milliseconds.
	if albRecord.RequestProcessingTime != unknownField {
		rpt, e := safeConvertStrToFloat(albRecord.RequestProcessingTime)
		if e == nil {
			recordLog.Attributes().PutDouble(AttributeELBRequestProcessingTime, rpt)
		}
	}
	if albRecord.TargetProcessingTime != unknownField {
		tpt, e := safeConvertStrToFloat(albRecord.TargetProcessingTime)
		if e == nil {
			recordLog.Attributes().PutDouble(AttributeELBTargetProcessingTime, tpt)
		}
	}
	if albRecord.ResponseProcessingTime != unknownField {
		rpt, e := safeConvertStrToFloat(albRecord.ResponseProcessingTime)
		if e == nil {
			recordLog.Attributes().PutDouble(AttributeELBResponseProcessingTime, rpt)
		}
	}

	if albRecord.TraceID != unknownField {
		recordLog.Attributes().PutStr(AttributeELBAWSTraceID, albRecord.TraceID)
	}
	if albRecord.TargetStatusCode != unknownField {
		statusCode, e := safeConvertStrToInt(albRecord.TargetStatusCode)
		if e == nil {
			recordLog.Attributes().PutInt(AttributeELBBackendStatusCode, statusCode)
		}
	}
	if albRecord.TargetGroupARN != unknownField {
		recordLog.Attributes().PutStr(AttributeELBTargetGroupARN, albRecord.TargetGroupARN)
	}
	if albRecord.ChosenCertARN != unknownField {
		recordLog.Attributes().PutStr(AttributeELBChosenCertARN, albRecord.ChosenCertARN)
	}
	if albRecord.ActionsExecuted != unknownField {
		actions := recordLog.Attributes().PutEmptySlice(AttributeELBActionsExecuted)
		for action := range strings.SplitSeq(albRecord.ActionsExecuted, ",") {
			actions.AppendEmpty().SetStr(action)
		}
	}
	if albRecord.RedirectURL != unknownField {
		recordLog.Attributes().PutStr(AttributeELBRedirectURL, albRecord.RedirectURL)
	}
	if albRecord.ErrorReason != unknownField {
		recordLog.Attributes().PutStr(AttributeELBErrorReason, albRecord.ErrorReason)
	}
	if albRecord.Classification != unknownField {
		recordLog.Attributes().PutStr(AttributeELBClassification, albRecord.Classification)
	}
	if albRecord.ClassificationReason != unknownField {
		recordLog.Attributes().PutStr(AttributeELBClassificationReason, albRecord.ClassificationReason)
	}
	if albRecord.ConnectionTraceID != unknownField {
		recordLog.Attributes().PutStr(AttributeELBConnectionTraceID, albRecord.ConnectionTraceID)
	}
	if albRecord.TransformedHost != unknownField {
		recordLog.Attributes().PutStr(AttributeELBTransformedHost, albRecord.TransformedHost)
	}
	if albRecord.TransformedURI != unknownField {
		recordLog.Attributes().PutStr(AttributeELBTransformedURI, albRecord.TransformedURI)
	}
	if albRecord.RequestTransformStatus != unknownField {
		recordLog.Attributes().PutStr(AttributeELBRequestTransformStatus, albRecord.RequestTransformStatus)
	}

	// Set timestamp
	recordLog.SetTimestamp(pcommon.Timestamp(epochNanoseconds))

	// move recordLog to scope
	rScope := scopeLogs.LogRecords().AppendEmpty()
	recordLog.MoveTo(rScope)
}

// handleNLBAccessLogs handles nlb access logs
func (f *elbAccessLogUnmarshaler) handleNLBAccessLogs(fields []string, resourceAttr *resourceAttributes, scopeLogs plog.ScopeLogs) error {
	record, err := convertTextToNLBAccessLogRecord(fields)
	if err != nil {
		return fmt.Errorf("unable to convert log line to ALB record: %w", err)
	}
	f.addToNLBAccessLogs(resourceAttr, scopeLogs, record)
	return nil
}

// addToNLBAccessLogs adds nlb record to provided logs based
// on the extracted logs of each resource
func (f *elbAccessLogUnmarshaler) addToNLBAccessLogs(resourceAttr *resourceAttributes, scopeLogs plog.ScopeLogs, nlbRecord NLBAccessLogRecord) {
	// Convert timestamp first; if invalid, skip log creation
	epochNanoseconds, err := convertToUnixEpoch(nlbRecord.Time)
	if err != nil {
		f.logger.Debug("Timestamp cannot be converted to unix epoch nanoseconds", zap.Error(err))
		return
	}

	// Create record log
	recordLog := plog.NewLogRecord()
	// Set resource id
	resourceAttr.resourceID = nlbRecord.ELB
	// Populate record attributes
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), nlbRecord.Type)
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolVersionKey), nlbRecord.Version)
	recordLog.Attributes().PutStr(string(conventions.ClientAddressKey), nlbRecord.ClientIP)
	recordLog.Attributes().PutInt(string(conventions.ClientPortKey), nlbRecord.ClientPort)
	recordLog.Attributes().PutStr(string(conventions.DestinationAddressKey), nlbRecord.DestinationIP)
	recordLog.Attributes().PutInt(string(conventions.DestinationPortKey), nlbRecord.DestinationPort)
	recordLog.Attributes().PutInt(string(conventions.HTTPRequestSizeKey), nlbRecord.ReceivedBytes)
	recordLog.Attributes().PutInt(string(conventions.HTTPResponseSizeKey), nlbRecord.SentBytes)
	recordLog.Attributes().PutStr(AttributeTLSListenerResourceID, nlbRecord.Listener)
	recordLog.Attributes().PutInt(AttributeELBConnectionTime, nlbRecord.ConnectionTime)
	recordLog.Attributes().PutInt(AttributeELBTLSHandshakeTime, nlbRecord.TLSHandshakeTime)
	recordLog.Attributes().PutStr(AttributeELBTLSConnectionCreationTime, nlbRecord.TLSConnectionCreationTime)

	// Attributes below may be unset (set to "-") in logs

	if nlbRecord.IncomingTLSAlert != unknownField {
		recordLog.Attributes().PutStr(AttributeELBIncomingTLSAlert, nlbRecord.IncomingTLSAlert)
	}

	if nlbRecord.ChosenCertARN != unknownField {
		recordLog.Attributes().PutStr(AttributeELBChosenCertARN, nlbRecord.ChosenCertARN)
	}

	if nlbRecord.ChosenCertSerial != unknownField {
		recordLog.Attributes().PutStr(AttributeELBChosenCertSerial, nlbRecord.ChosenCertSerial)
	}

	if nlbRecord.TLSCipher != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSCipherKey), nlbRecord.TLSCipher)
	}

	if nlbRecord.TLSProtocolVersion != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSProtocolVersionKey), nlbRecord.TLSProtocolVersion)
	}

	if nlbRecord.TLSNamedGroup != unknownField {
		recordLog.Attributes().PutStr(AttributeELBTLSNamedGroup, nlbRecord.TLSNamedGroup)
	}

	if nlbRecord.DomainName != unknownField {
		recordLog.Attributes().PutStr(string(conventions.URLDomainKey), nlbRecord.DomainName)
	}

	if nlbRecord.ALPNFeProtocol != unknownField {
		recordLog.Attributes().PutStr(AttributeALPNFeProtocol, nlbRecord.ALPNFeProtocol)
	}

	if nlbRecord.ALPNBeProtocol != unknownField {
		recordLog.Attributes().PutStr(AttributeALPNBeProtocol, nlbRecord.ALPNBeProtocol)
	}

	if nlbRecord.ALPNClientPreferenceList != unknownField {
		splits := strings.Split(nlbRecord.ALPNClientPreferenceList, ",")
		slice := recordLog.Attributes().PutEmptySlice(AttributeALPNClientPreferenceList)

		for _, split := range splits {
			slice.AppendEmpty().SetStr(split)
		}
	}

	// Set timestamp
	recordLog.SetTimestamp(pcommon.Timestamp(epochNanoseconds))

	// move recordLog to scope
	rScope := scopeLogs.LogRecords().AppendEmpty()
	recordLog.MoveTo(rScope)
}
