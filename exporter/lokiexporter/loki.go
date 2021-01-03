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

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type lokiExporter struct {
	config                 *Config
	logger                 *zap.Logger
	client                 *http.Client
	attributesToLabelNames *map[string]model.LabelName
	wg                     sync.WaitGroup
}

func newExporter(config *Config, logger *zap.Logger) (*lokiExporter, error) {
	client, err := config.HTTPClientSettings.ToClient()
	if err != nil {
		return nil, err
	}

	return &lokiExporter{
		config:                 config,
		logger:                 logger,
		client:                 client,
		attributesToLabelNames: config.Labels.getAttributesToLabelNames(),
	}, nil
}

func (l *lokiExporter) pushLogData(ctx context.Context, ld pdata.Logs) (numDroppedLogs int, err error) {
	l.wg.Add(1)
	defer l.wg.Done()

	pushReq, numDroppedLogs := logDataToLoki(l.logger, ld, l.attributesToLabelNames)
	if len(pushReq.Streams) == 0 {
		return numDroppedLogs, nil
	}

	buf, err := encodePushRequest(pushReq)
	if err != nil {
		numDroppedLogs += ld.LogRecordCount()
		return numDroppedLogs, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.config.HTTPClientSettings.Endpoint, bytes.NewReader(buf))
	if err != nil {
		return numDroppedLogs, consumererror.Permanent(err)
	}

	for k, v := range l.config.HTTPClientSettings.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")

	resp, err := l.client.Do(req)
	if err != nil {
		numDroppedLogs += ld.LogRecordCount()
		return numDroppedLogs, err
	}

	_, _ = io.Copy(ioutil.Discard, resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf("HTTP %d %q", resp.StatusCode, http.StatusText(resp.StatusCode))
		return numDroppedLogs, err
	}

	return numDroppedLogs, nil
}

func encodePushRequest(pushRequest *logproto.PushRequest) ([]byte, error) {
	buf, err := proto.Marshal(pushRequest)
	if err != nil {
		return nil, err
	}
	buf = snappy.Encode(nil, buf)
	return buf, nil
}

func (l *lokiExporter) start(ctx context.Context, host component.Host) (err error) {
	return nil
}

func (l *lokiExporter) stop(ctx context.Context) (err error) {
	l.wg.Wait()
	return nil
}
