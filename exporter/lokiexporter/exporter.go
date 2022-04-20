// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/third_party/loki/logproto"
)

const (
	maxErrMsgLen = 1024
)

type lokiExporter struct {
	config   *Config
	settings component.TelemetrySettings
	client   *http.Client
	wg       sync.WaitGroup
	convert  func(plog.LogRecord, pcommon.Resource) (*logproto.Entry, error)
}

func newExporter(config *Config, settings component.TelemetrySettings) *lokiExporter {
	lokiexporter := &lokiExporter{
		config:   config,
		settings: settings,
	}
	if config.Format == "json" {
		lokiexporter.convert = lokiexporter.convertLogToJSONEntry
	} else {
		lokiexporter.convert = lokiexporter.convertLogBodyToEntry
	}
	return lokiexporter
}

func (l *lokiExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	l.wg.Add(1)
	defer l.wg.Done()

	pushReq, _ := l.logDataToLoki(ld)
	if len(pushReq.Streams) == 0 {
		return consumererror.NewPermanent(fmt.Errorf("failed to transform logs into Loki log streams"))
	}

	buf, err := encode(pushReq)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.config.HTTPClientSettings.Endpoint, bytes.NewReader(buf))
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	for k, v := range l.config.HTTPClientSettings.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")

	if len(l.config.TenantID) > 0 {
		req.Header.Set("X-Scope-OrgID", l.config.TenantID)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return consumererror.NewLogs(err, ld)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("HTTP %d %q: %s", resp.StatusCode, http.StatusText(resp.StatusCode), line)
		return consumererror.NewLogs(err, ld)
	}

	return nil
}

func encode(pb proto.Message) ([]byte, error) {
	buf, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	buf = snappy.Encode(nil, buf)
	return buf, nil
}

func (l *lokiExporter) start(_ context.Context, host component.Host) (err error) {
	client, err := l.config.HTTPClientSettings.ToClient(host.GetExtensions(), l.settings)
	if err != nil {
		return err
	}

	l.client = client

	return nil
}

func (l *lokiExporter) stop(context.Context) (err error) {
	l.wg.Wait()
	return nil
}

func (l *lokiExporter) logDataToLoki(ld plog.Logs) (pr *logproto.PushRequest, numDroppedLogs int) {
	var errs error

	streams := make(map[string]*logproto.Stream)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()
		resource := rls.At(i).Resource()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)

				mergedLabels, dropped := l.convertAttributesAndMerge(log.Attributes(), resource.Attributes())
				if dropped {
					numDroppedLogs++
					continue
				}

				// now merge the labels based on the record attributes
				recordLabels := l.convertRecordAttributesToLabels(log)
				mergedLabels = mergedLabels.Merge(recordLabels)

				labels := mergedLabels.String()
				var entry *logproto.Entry
				var err error
				entry, err = l.convert(log, resource)
				if err != nil {
					// Couldn't convert so dropping log.
					numDroppedLogs++
					errs = multierr.Append(
						errs,
						errors.New(
							fmt.Sprint(
								"failed to convert, dropping log",
								zap.String("format", l.config.Format),
								zap.Error(err),
							),
						),
					)
					continue
				}

				if stream, ok := streams[labels]; ok {
					stream.Entries = append(stream.Entries, *entry)
					continue
				}

				streams[labels] = &logproto.Stream{
					Labels:  labels,
					Entries: []logproto.Entry{*entry},
				}
			}
		}
	}

	if errs != nil {
		l.settings.Logger.Debug("some logs has been dropped", zap.Error(errs))
	}

	pr = &logproto.PushRequest{
		Streams: make([]logproto.Stream, len(streams)),
	}

	i := 0
	for _, stream := range streams {
		pr.Streams[i] = *stream
		i++
	}

	return pr, numDroppedLogs
}

func (l *lokiExporter) convertAttributesAndMerge(logAttrs pcommon.Map, resourceAttrs pcommon.Map) (mergedAttributes model.LabelSet, dropped bool) {
	logRecordAttributes := l.convertAttributesToLabels(logAttrs, l.config.Labels.Attributes)
	resourceAttributes := l.convertAttributesToLabels(resourceAttrs, l.config.Labels.ResourceAttributes)

	// This prometheus model.labelset Merge function overwrites	the logRecordAttributes with resourceAttributes
	mergedAttributes = logRecordAttributes.Merge(resourceAttributes)

	if len(mergedAttributes) == 0 {
		return nil, true
	}
	return mergedAttributes, false
}

func (l *lokiExporter) convertAttributesToLabels(attributes pcommon.Map, allowedAttributes map[string]string) model.LabelSet {
	ls := model.LabelSet{}

	allowedLabels := l.config.Labels.getAttributes(allowedAttributes)

	for attr, attrLabelName := range allowedLabels {
		av, ok := attributes.Get(attr)
		if ok {
			if av.Type() != pcommon.ValueTypeString {
				l.settings.Logger.Debug("Failed to convert attribute value to Loki label value, value is not a string", zap.String("attribute", attr))
				continue
			}
			ls[attrLabelName] = model.LabelValue(av.StringVal())
		}
	}

	return ls
}

func (l *lokiExporter) convertRecordAttributesToLabels(log plog.LogRecord) model.LabelSet {
	ls := model.LabelSet{}

	if val, ok := l.config.Labels.RecordAttributes["traceID"]; ok {
		ls[model.LabelName(val)] = model.LabelValue(log.TraceID().HexString())
	}

	if val, ok := l.config.Labels.RecordAttributes["spanID"]; ok {
		ls[model.LabelName(val)] = model.LabelValue(log.SpanID().HexString())
	}

	if val, ok := l.config.Labels.RecordAttributes["severity"]; ok {
		ls[model.LabelName(val)] = model.LabelValue(log.SeverityText())
	}

	if val, ok := l.config.Labels.RecordAttributes["severityN"]; ok {
		ls[model.LabelName(val)] = model.LabelValue(log.SeverityNumber().String())
	}

	return ls
}

func (l *lokiExporter) convertLogBodyToEntry(lr plog.LogRecord, res pcommon.Resource) (*logproto.Entry, error) {
	var b strings.Builder

	if _, ok := l.config.Labels.RecordAttributes["severity"]; !ok && len(lr.SeverityText()) > 0 {
		b.WriteString("severity=")
		b.WriteString(lr.SeverityText())
		b.WriteRune(' ')
	}
	if _, ok := l.config.Labels.RecordAttributes["severityN"]; !ok && lr.SeverityNumber() > 0 {
		b.WriteString("severityN=")
		b.WriteString(strconv.Itoa(int(lr.SeverityNumber())))
		b.WriteRune(' ')
	}
	if _, ok := l.config.Labels.RecordAttributes["traceID"]; !ok && !lr.TraceID().IsEmpty() {
		b.WriteString("traceID=")
		b.WriteString(lr.TraceID().HexString())
		b.WriteRune(' ')
	}
	if _, ok := l.config.Labels.RecordAttributes["spanID"]; !ok && !lr.SpanID().IsEmpty() {
		b.WriteString("spanID=")
		b.WriteString(lr.SpanID().HexString())
		b.WriteRune(' ')
	}

	// fields not added to the accept-list as part of the component's config
	// are added to the body, so that they can still be seen under "detected fields"
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		if _, found := l.config.Labels.Attributes[k]; !found {
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(v.AsString())
			b.WriteRune(' ')
		}
		return true
	})

	// same for resources: include all, except the ones that are explicitly added
	// as part of the config, which are showing up at the top-level already
	res.Attributes().Range(func(k string, v pcommon.Value) bool {
		if _, found := l.config.Labels.ResourceAttributes[k]; !found {
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(v.AsString())
			b.WriteRune(' ')
		}
		return true
	})

	b.WriteString(lr.Body().StringVal())

	return &logproto.Entry{
		Timestamp: timestampFromLogRecord(lr),
		Line:      b.String(),
	}, nil
}

func (l *lokiExporter) convertLogToJSONEntry(lr plog.LogRecord, res pcommon.Resource) (*logproto.Entry, error) {
	line, err := encodeJSON(lr, res)
	if err != nil {
		return nil, err
	}
	return &logproto.Entry{
		Timestamp: timestampFromLogRecord(lr),
		Line:      line,
	}, nil
}

func timestampFromLogRecord(lr plog.LogRecord) time.Time {
	if lr.Timestamp() == 0 {
		return time.Unix(0, int64(lr.ObservedTimestamp()))
	}
	return time.Unix(0, int64(lr.Timestamp()))
}
