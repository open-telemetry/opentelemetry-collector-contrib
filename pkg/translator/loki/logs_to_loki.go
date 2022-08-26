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

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"errors"
	"fmt"

	"github.com/grafana/loki/pkg/logproto"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

func LogsToLoki(logger *zap.Logger, ld plog.Logs) (pr *logproto.PushRequest) {
	var errs error

	streams := make(map[string]*logproto.Stream)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()

		// we may remove attributes, so we make a copy and change our version
		resource := pcommon.NewResource()
		rls.At(i).Resource().CopyTo(resource)

		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {

				// similarly, we may remove attributes, so change only our version
				log := plog.NewLogRecord()
				logs.At(k).CopyTo(log)

				mergedLabels := convertAttributesAndMerge(log.Attributes(), resource.Attributes())
				// remove the attributes that were promoted to labels
				removeAttributes(log.Attributes(), mergedLabels)
				removeAttributes(resource.Attributes(), mergedLabels)

				// create the stream name based on the labels
				labels := mergedLabels.String()

				entry, err := convertLogToJSONEntry(log, resource)
				if err != nil {
					// Couldn't convert so dropping log.
					errs = multierr.Append(
						errs,
						errors.New(
							fmt.Sprint(
								"failed to convert, dropping log",
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
		logger.Debug("some logs has been dropped", zap.Error(errs))
	}

	pr = &logproto.PushRequest{
		Streams: make([]logproto.Stream, len(streams)),
	}

	i := 0
	for _, stream := range streams {
		pr.Streams[i] = *stream
		i++
	}

	return pr
}
