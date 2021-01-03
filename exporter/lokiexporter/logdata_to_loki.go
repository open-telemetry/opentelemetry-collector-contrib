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
	"time"

	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func logDataToLoki(logger *zap.Logger, ld pdata.Logs, allowedAttributesToLabels *map[string]model.LabelName) (pr *logproto.PushRequest, numDroppedLogs int) {
	streams := make(map[string]*logproto.Stream)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).Logs()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				labels, ok := convertAllowedAttributesToLabels(log.Attributes(), allowedAttributesToLabels, logger)
				if !ok {
					numDroppedLogs++
					continue
				}

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

	for _, stream := range streams {
		pr.Streams = append(pr.Streams, *stream)
	}

	return pr, numDroppedLogs
}

func convertAllowedAttributesToLabels(attributes pdata.AttributeMap, allowedAttributesToLabels *map[string]model.LabelName, logger *zap.Logger) (string, bool) {
	ls := model.LabelSet{}

	for attr, attrLabelName := range *allowedAttributesToLabels {
		av, ok := attributes.Get(attr)
		if ok {
			if av.Type() != pdata.AttributeValueSTRING {
				logger.Debug("Failed to convert attribute value to Loki label value, value is not a string", zap.String("attribute", attr))
				continue
			}
			ls[attrLabelName] = model.LabelValue(av.StringVal())
		}
	}

	labels := ls.String()

	// Calling String() on an empty LabelSet returns "{}".
	if labels == "{}" {
		return labels, false
	}

	return labels, true
}

func convertLogToLokiEntry(lr pdata.LogRecord) *logproto.Entry {
	return &logproto.Entry{
		Timestamp: time.Unix(0, int64(lr.Timestamp())),
		Line:      lr.Body().StringVal(),
	}
}
