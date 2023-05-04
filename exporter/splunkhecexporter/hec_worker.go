// Copyright The OpenTelemetry Authors
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

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

type hecWorker interface {
	send(context.Context, *bufferState, map[string]string) error
}

type defaultHecWorker struct {
	url     *url.URL
	client  *http.Client
	headers map[string]string
}

func (hec *defaultHecWorker) send(ctx context.Context, bufferState *bufferState, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", hec.url.String(), bufferState)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	req.ContentLength = int64(bufferState.buf.Len())

	// Set the headers configured for the client
	for k, v := range hec.headers {
		req.Header.Set(k, v)
	}

	// Set extra headers passed by the caller
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if bufferState.compressionEnabled {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := hec.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}

	// Do not drain the response when 429 or 502 status code is returned.
	// HTTP client will not reuse the same connection unless it is drained.
	// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18281 for more details.
	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusBadGateway {
		_, errCopy := io.Copy(io.Discard, resp.Body)
		err = multierr.Combine(err, errCopy)
	}
	return err
}

var _ hecWorker = &defaultHecWorker{}
