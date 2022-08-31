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
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/third_party/loki/logproto"
)

type nextLokiExporter struct {
	config   *Config
	settings component.TelemetrySettings
	client   *http.Client
	wg       sync.WaitGroup
}

func newNextExporter(config *Config, settings component.TelemetrySettings) *nextLokiExporter {
	settings.Logger.Info("using the new Loki exporter")

	return &nextLokiExporter{
		config:   config,
		settings: settings,
	}
}

func (l *nextLokiExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	pushReq := logDataToLoki(l.settings.Logger, ld)
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

func (l *nextLokiExporter) start(_ context.Context, host component.Host) (err error) {
	client, err := l.config.HTTPClientSettings.ToClient(host, l.settings)
	if err != nil {
		return err
	}

	l.client = client

	return nil
}

func (l *nextLokiExporter) stop(context.Context) (err error) {
	l.wg.Wait()
	return nil
}

func logDataToLoki(logger *zap.Logger, ld plog.Logs) (pr *logproto.PushRequest) {
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
