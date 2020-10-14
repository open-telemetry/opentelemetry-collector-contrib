// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signalfxexporter

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

// sfxEventClient sends the data to the SignalFx backend.
type sfxEventClient struct {
	sfxClientBase
	logger                 *zap.Logger
	accessTokenPassthrough bool
}

func (s *sfxEventClient) pushResourceLogs(
	ctx context.Context,
	rls pdata.ResourceLogs,
) (int, error) {
	accessToken := s.retrieveAccessToken(rls)

	var sfxEvents []*sfxpb.Event
	numDroppedLogRecords := 0

	ills := rls.InstrumentationLibraryLogs()
	for j := 0; j < ills.Len(); j++ {
		ill := ills.At(j)

		events, dropped := translation.LogSliceToSignalFxV2(s.logger, ill.Logs())
		sfxEvents = append(sfxEvents, events...)
		numDroppedLogRecords += dropped
	}

	body, compressed, err := s.encodeBody(sfxEvents)
	if err != nil {
		return countRecords(rls), consumererror.Permanent(err)
	}

	eventURL := *s.ingestURL
	if !strings.HasSuffix(eventURL.Path, "v2/event") {
		eventURL.Path = path.Join(eventURL.Path, "v2/event")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", eventURL.String(), body)
	if err != nil {
		return countRecords(rls), consumererror.Permanent(err)
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	if s.accessTokenPassthrough && accessToken != "" {
		req.Header.Set(splunk.SFxAccessTokenHeader, accessToken)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return countRecords(rls), err
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	// SignalFx accepts all 2XX codes.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := ioutil.ReadAll(resp.Body)
		err = fmt.Errorf(
			"HTTP %d %q for %q: %s",
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
			req.URL,
			body)
		return countRecords(rls), err
	}

	return numDroppedLogRecords, nil
}

func (s *sfxEventClient) encodeBody(events []*sfxpb.Event) (bodyReader io.Reader, compressed bool, err error) {
	msg := sfxpb.EventUploadMessage{
		Events: events,
	}
	body, err := msg.Marshal()
	if err != nil {
		return nil, false, err
	}
	return s.getReader(body)
}

func (s *sfxEventClient) retrieveAccessToken(rl pdata.ResourceLogs) string {
	if rl.IsNil() || rl.Resource().IsNil() {
		return ""
	}
	attrs := rl.Resource().Attributes()
	if accessToken, ok := attrs.Get(splunk.SFxAccessTokenLabel); ok && accessToken.Type() == pdata.AttributeValueSTRING {
		// Drop internally passed access token in all cases
		attrs.Delete(splunk.SFxAccessTokenLabel)
		return accessToken.StringVal()
	}
	return ""
}

func countRecords(rs pdata.ResourceLogs) int {
	logCount := 0

	ill := rs.InstrumentationLibraryLogs()
	for i := 0; i < ill.Len(); i++ {
		logs := ill.At(i)
		logCount += logs.Logs().Len()
	}
	return logCount
}
