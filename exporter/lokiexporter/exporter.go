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

package lokiexporter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/third_party/loki/logproto"
)

type lokiExporter struct {
	config *Config
	logger *zap.Logger
	client *http.Client
	wg     sync.WaitGroup
}

func newExporter(config *Config, logger *zap.Logger) *lokiExporter {
	return &lokiExporter{
		config: config,
		logger: logger,
	}
}

func (l *lokiExporter) pushLogData(ctx context.Context, ld pdata.Logs) error {
	l.wg.Add(1)
	defer l.wg.Done()

	pushReq, _ := l.logDataToLoki(ld)
	if len(pushReq.Streams) == 0 {
		return consumererror.Permanent(fmt.Errorf("failed to transform logs into Loki log streams"))
	}

	buf, err := encode(pushReq)
	if err != nil {
		return consumererror.Permanent(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.config.HTTPClientSettings.Endpoint, bytes.NewReader(buf))
	if err != nil {
		return consumererror.Permanent(err)
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

	_, _ = io.Copy(ioutil.Discard, resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf("HTTP %d %q", resp.StatusCode, http.StatusText(resp.StatusCode))
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
	client, err := l.config.HTTPClientSettings.ToClient(host.GetExtensions())
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

func (l *lokiExporter) logDataToLoki(ld pdata.Logs) (pr *logproto.PushRequest, numDroppedLogs int) {
	streams := make(map[string]*logproto.Stream)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).InstrumentationLibraryLogs()
		resource := rls.At(i).Resource()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).Logs()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)

				mergedLabels, dropped := l.convertAttributesAndMerge(log.Attributes(), resource.Attributes())
				if dropped {
					numDroppedLogs++
					continue
				}
				labels := mergedLabels.String()
				entry := convertLogToLokiEntry(log)

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

func (l *lokiExporter) convertAttributesAndMerge(logAttrs pdata.AttributeMap, resourceAttrs pdata.AttributeMap) (mergedAttributes model.LabelSet, dropped bool) {
	logRecordAttributes := l.convertAttributesToLabels(logAttrs, l.config.Labels.Attributes)
	resourceAttributes := l.convertAttributesToLabels(resourceAttrs, l.config.Labels.ResourceAttributes)

	// This prometheus model.labelset Merge function overwrites	the logRecordAttributes with resourceAttributes
	mergedAttributes = logRecordAttributes.Merge(resourceAttributes)

	if len(mergedAttributes) == 0 {
		return nil, true
	}
	return mergedAttributes, false
}

func (l *lokiExporter) convertAttributesToLabels(attributes pdata.AttributeMap, allowedAttributes map[string]string) model.LabelSet {
	ls := model.LabelSet{}

	allowedLabels := l.config.Labels.getAttributes(allowedAttributes)

	for attr, attrLabelName := range allowedLabels {
		av, ok := attributes.Get(attr)
		if ok {
			if av.Type() != pdata.AttributeValueTypeString {
				l.logger.Debug("Failed to convert attribute value to Loki label value, value is not a string", zap.String("attribute", attr))
				continue
			}
			ls[attrLabelName] = model.LabelValue(av.StringVal())
		}
	}

	return ls
}

func convertLogToLokiEntry(lr pdata.LogRecord) *logproto.Entry {
	return &logproto.Entry{
		Timestamp: time.Unix(0, int64(lr.Timestamp())),
		Line:      lr.Body().StringVal(),
	}
}
