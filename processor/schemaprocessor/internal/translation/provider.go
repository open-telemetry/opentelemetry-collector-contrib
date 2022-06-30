// Copyright  The OpenTelemetry Authors
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

package translation

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

type (
	// Provider allows for collector extensions to be used to look up schemaURLs
	Provider interface {
		// Lookup whill check the underlying provider to see if content exists
		// for the provided schemaURL, in the even that it doesn't an error is returned.
		Lookup(ctx context.Context, schemaURL string) (content io.Reader, err error)
	}

	httpProvider struct {
		client *http.Client
	}
)

var (
	_ Provider = (*httpProvider)(nil)
)

func NewHTTPProvider(client *http.Client) Provider {
	return &httpProvider{client: client}
}

func (hp *httpProvider) Lookup(ctx context.Context, schemaURL string) (io.Reader, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, schemaURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := hp.client.Do(req)
	if err != nil {
		return nil, err
	}
	content := bytes.NewBuffer(nil)
	if _, err := content.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("invalid status code returned: %d", resp.StatusCode)
	}
	return content, nil
}
