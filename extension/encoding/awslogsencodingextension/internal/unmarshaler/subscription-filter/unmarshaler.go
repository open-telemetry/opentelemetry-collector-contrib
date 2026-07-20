// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
)

const ctrlMessageType = "CONTROL_MESSAGE"

var (
	errEmptyOwner     = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty owner field")
	errEmptyLogGroup  = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty log group field")
	errEmptyLogStream = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty log stream field")
)

var _ unmarshaler.StreamingLogsUnmarshaler = (*SubscriptionFilterUnmarshaler)(nil)

// route is a resolved CloudWatchStream: patterns are pre-split, the inner
// encoding extension is already type-asserted, and payload is non-empty.
type route struct {
	name           string
	inner          plog.Unmarshaler
	logGroupParts  []string
	logStreamParts []string
	payload        PayloadMode
}

type SubscriptionFilterUnmarshaler struct {
	buildInfo component.BuildInfo
	routes    []route
}

func NewSubscriptionFilterUnmarshaler(buildInfo component.BuildInfo) *SubscriptionFilterUnmarshaler {
	return &SubscriptionFilterUnmarshaler{
		buildInfo: buildInfo,
	}
}

// ConfigureRoutes resolves the user's CloudWatchStream config against the
// host's loaded extensions and installs the resulting route table for
// per-envelope dispatch. Called from the extension's Start.
func (f *SubscriptionFilterUnmarshaler) ConfigureRoutes(streams []CloudWatchStream, host component.Host, selfID component.ID) error {
	routes, err := resolveStreams(streams, host, selfID)
	if err != nil {
		return err
	}
	f.routes = routes
	return nil
}

// UnmarshalAWSLogs deserializes the given reader as CloudWatch Logs events
// into a plog.Logs, grouping logs by owner (account ID), log group, and
// log stream. When extracted fields are present (from centralized logging),
// logs are further grouped by their extracted account ID and region.
// Logs are assumed to be gzip-compressed as specified at
// https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html.
func (f *SubscriptionFilterUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	// Decode as a stream but flush all at once using flush options
	streamUnmarshaler, err := f.NewLogsDecoder(reader, encoding.WithFlushItems(0), encoding.WithFlushBytes(0))
	if err != nil {
		return plog.Logs{}, err
	}
	logs, err := streamUnmarshaler.DecodeLogs()
	if err != nil {
		// we must check for EOF with direct comparison and avoid wrapped EOF that can come from stream itself
		//nolint:errorlint
		if err == io.EOF {
			// EOF indicates no logs were found, return any logs that's available
			return logs, nil
		}

		return plog.Logs{}, err
	}

	return logs, nil
}

// NewLogsDecoder returns a LogsDecoder that processes CloudWatch Logs subscription filter events.
// Supported sub formats:
//   - DATA_MESSAGE: Returns logs grouped by owner, log group, and stream; offset is the number of records processed
//   - CONTROL_MESSAGE: Returns empty log; offset is the number of records processed
func (f *SubscriptionFilterUnmarshaler) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	batchHelper := xstreamencoding.NewBatchHelper(options...)
	decoder := gojson.NewDecoder(reader)

	var offset int64

	if batchHelper.Options().Offset > 0 {
		for offset < batchHelper.Options().Offset {
			if !decoder.More() {
				return nil, fmt.Errorf("EOF reached before offset %d records were discarded", batchHelper.Options().Offset)
			}

			var raw gojson.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				return nil, err
			}
			offset++
		}
	}

	return xstreamencoding.NewLogsDecoderAdapter(
		func() (plog.Logs, error) {
			logs := plog.NewLogs()
			resourceLogsByKey := make(map[resourceGroupKey]plog.LogRecordSlice)

			for decoder.More() {
				if err := f.decodeOne(decoder, logs, resourceLogsByKey); err != nil {
					return plog.Logs{}, err
				}

				offset++
				batchHelper.IncrementItems(1)

				if batchHelper.ShouldFlush() {
					batchHelper.Reset()
					return logs, nil
				}
			}

			if logs.ResourceLogs().Len() == 0 {
				return plog.NewLogs(), io.EOF
			}

			return logs, nil
		}, func() int64 {
			return offset
		},
	), nil
}

// decodeOne reads one CloudWatch envelope from decoder. With no routes
// configured, the envelope is decoded once into cloudwatchLogsData and
// merged via appendLogs (existing behavior). With routes, the envelope is
// captured as raw bytes, the header is peeked to drive route selection, and
// dispatch happens per envelope.
func (f *SubscriptionFilterUnmarshaler) decodeOne(
	decoder *gojson.Decoder,
	logs plog.Logs,
	resourceLogsByKey map[resourceGroupKey]plog.LogRecordSlice,
) error {
	if len(f.routes) == 0 {
		var cwLog cloudwatchLogsData
		if err := decoder.Decode(&cwLog); err != nil {
			return fmt.Errorf("failed to decode decompressed reader: %w", err)
		}
		if cwLog.MessageType == ctrlMessageType {
			return nil
		}
		if err := validateLog(cwLog); err != nil {
			return fmt.Errorf("invalid cloudwatch log: %w", err)
		}
		f.appendLogs(logs, resourceLogsByKey, cwLog)
		return nil
	}

	var raw gojson.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("failed to decode decompressed reader: %w", err)
	}

	var hdr cloudwatchLogsHeader
	if err := gojson.Unmarshal(raw, &hdr); err != nil {
		return fmt.Errorf("failed to decode decompressed reader: %w", err)
	}
	if hdr.MessageType == ctrlMessageType {
		return nil
	}
	if err := validateLogFields(hdr.MessageType, hdr.Owner, hdr.LogGroup, hdr.LogStream); err != nil {
		return fmt.Errorf("invalid cloudwatch log: %w", err)
	}

	r := f.matchRoute(hdr.LogGroup, hdr.LogStream)

	// Envelope dispatch hands the raw bytes directly to the inner encoding; no full parse needed.
	if r != nil && r.payload == PayloadEnvelope {
		return f.dispatchEnvelope(logs, []byte(raw), hdr.LogGroup, *r)
	}

	// Both the no-route fallback and message dispatch need the parsed envelope.
	var cwLog cloudwatchLogsData
	if err := gojson.Unmarshal(raw, &cwLog); err != nil {
		return fmt.Errorf("failed to decode decompressed reader: %w", err)
	}
	if r == nil {
		f.appendLogs(logs, resourceLogsByKey, cwLog)
		return nil
	}
	return f.dispatchMessage(logs, cwLog, *r)
}

func (f *SubscriptionFilterUnmarshaler) matchRoute(logGroup, logStream string) *route {
	var groupParts, streamParts []string
	for i := range f.routes {
		r := &f.routes[i]
		if r.logGroupParts != nil {
			if groupParts == nil {
				groupParts = strings.Split(logGroup, "/")
			}
			if matchPrefixWithWildcard(groupParts, r.logGroupParts) {
				return r
			}
		}
		if r.logStreamParts != nil {
			if streamParts == nil {
				streamParts = strings.Split(logStream, "/")
			}
			if matchPrefixWithWildcard(streamParts, r.logStreamParts) {
				return r
			}
		}
	}
	return nil
}

// dispatchEnvelope hands the raw CloudWatch envelope bytes to the inner
// encoding once. The inner is responsible for parsing and resource attrs.
func (*SubscriptionFilterUnmarshaler) dispatchEnvelope(logs plog.Logs, raw []byte, logGroup string, r route) error {
	innerLogs, err := r.inner.UnmarshalLogs(raw)
	if err != nil {
		return fmt.Errorf("stream %q: inner encoding failed for log group %q: %w", r.name, logGroup, err)
	}
	innerLogs.ResourceLogs().MoveAndAppendTo(logs.ResourceLogs())
	return nil
}

// dispatchMessage hands each event's message bytes to the inner encoding,
// then attaches CloudWatch resource attrs and back-fills the event timestamp.
func (*SubscriptionFilterUnmarshaler) dispatchMessage(logs plog.Logs, cwLog cloudwatchLogsData, r route) error {
	for _, event := range cwLog.LogEvents {
		innerLogs, err := r.inner.UnmarshalLogs([]byte(event.Message))
		if err != nil {
			return fmt.Errorf("stream %q: inner encoding failed on event %q: %w", r.name, event.ID, err)
		}
		eventTS := pcommon.Timestamp(event.Timestamp * int64(time.Millisecond))
		accountID, region := resolveAccountAndRegion(event, cwLog.Owner)
		for i := 0; i < innerLogs.ResourceLogs().Len(); i++ {
			rl := innerLogs.ResourceLogs().At(i)
			addCloudWatchResourceAttrs(rl.Resource().Attributes(), accountID, region, cwLog.LogGroup, cwLog.LogStream)
			backfillTimestamps(rl, eventTS)
		}
		innerLogs.ResourceLogs().MoveAndAppendTo(logs.ResourceLogs())
	}
	return nil
}

func addCloudWatchResourceAttrs(attrs pcommon.Map, accountID, region, logGroup, logStream string) {
	if _, ok := attrs.Get(string(conventions.CloudProviderKey)); !ok {
		attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	}
	if accountID != "" {
		if _, ok := attrs.Get(string(conventions.CloudAccountIDKey)); !ok {
			attrs.PutStr(string(conventions.CloudAccountIDKey), accountID)
		}
	}
	if region != "" {
		if _, ok := attrs.Get(string(conventions.CloudRegionKey)); !ok {
			attrs.PutStr(string(conventions.CloudRegionKey), region)
		}
	}
	attrs.PutEmptySlice(string(conventions.AWSLogGroupNamesKey)).AppendEmpty().SetStr(logGroup)
	attrs.PutEmptySlice(string(conventions.AWSLogStreamNamesKey)).AppendEmpty().SetStr(logStream)
}

func backfillTimestamps(rl plog.ResourceLogs, ts pcommon.Timestamp) {
	for i := 0; i < rl.ScopeLogs().Len(); i++ {
		sl := rl.ScopeLogs().At(i)
		for j := 0; j < sl.LogRecords().Len(); j++ {
			lr := sl.LogRecords().At(j)
			if lr.Timestamp() == 0 {
				lr.SetTimestamp(ts)
			}
		}
	}
}

func resolveAccountAndRegion(event cloudwatchLogsLogEvent, owner string) (string, string) {
	if event.ExtractedFields == nil {
		return owner, ""
	}
	accountID := event.ExtractedFields.AccountID
	if accountID == "" {
		accountID = owner
	}
	return accountID, event.ExtractedFields.Region
}

// appendLogs appends log records from cwLog into the given plog.Logs, reusing
// existing ResourceLogs entries tracked by resourceLogsByKey when possible.
// Events are grouped by their extracted fields (account ID + region) and
// by log group/stream combination.
func (f *SubscriptionFilterUnmarshaler) appendLogs(logs plog.Logs, resourceLogsByKey map[resourceGroupKey]plog.LogRecordSlice, cwLog cloudwatchLogsData) {
	for _, event := range cwLog.LogEvents {
		key := extractResourceKey(event, cwLog.Owner, cwLog.LogGroup, cwLog.LogStream)

		logRecords, exists := resourceLogsByKey[key]
		if !exists {
			rl := logs.ResourceLogs().AppendEmpty()
			resourceAttrs := rl.Resource().Attributes()
			resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
			resourceAttrs.PutStr(string(conventions.CloudAccountIDKey), key.accountID)
			if key.region != "" {
				resourceAttrs.PutStr(string(conventions.CloudRegionKey), key.region)
			}
			resourceAttrs.PutEmptySlice(string(conventions.AWSLogGroupNamesKey)).AppendEmpty().SetStr(cwLog.LogGroup)
			resourceAttrs.PutEmptySlice(string(conventions.AWSLogStreamNamesKey)).AppendEmpty().SetStr(cwLog.LogStream)

			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName(metadata.ScopeName)
			sl.Scope().SetVersion(f.buildInfo.Version)
			sl.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatCloudWatchLogsSubscriptionFilter)

			logRecords = sl.LogRecords()
			resourceLogsByKey[key] = logRecords
		}

		logRecord := logRecords.AppendEmpty()
		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logRecord.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
		logRecord.Body().SetStr(event.Message)
	}
}

// extractResourceKey extracts the resource group key from a log event.
// When extracted fields are present, uses those; otherwise falls back to owner.
func extractResourceKey(event cloudwatchLogsLogEvent, owner, logGroup, logStream string) resourceGroupKey {
	var key resourceGroupKey
	key.logGroup = logGroup
	key.logStream = logStream
	if event.ExtractedFields != nil {
		if event.ExtractedFields.AccountID != "" {
			key.accountID = event.ExtractedFields.AccountID
		} else {
			key.accountID = owner
		}
		key.region = event.ExtractedFields.Region
	} else {
		key.accountID = owner
	}
	return key
}

func validateLog(log cloudwatchLogsData) error {
	return validateLogFields(log.MessageType, log.Owner, log.LogGroup, log.LogStream)
}

func validateLogFields(messageType, owner, logGroup, logStream string) error {
	switch messageType {
	case "DATA_MESSAGE":
		if owner == "" {
			return errEmptyOwner
		}
		if logGroup == "" {
			return errEmptyLogGroup
		}
		if logStream == "" {
			return errEmptyLogStream
		}
	case ctrlMessageType:
	default:
		return fmt.Errorf("cloudwatch log has invalid message type %q", messageType)
	}
	return nil
}
