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

package healthchecker

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type HTTPHealthChecker struct {
	endpoint string
}

func NewHTTPHealthChecker(endpoint string) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		endpoint: endpoint,
	}
}

func (h *HTTPHealthChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", h.endpoint, nil)
	if err != nil {
		return err
	}

	client := http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check on %s returned %d", h.endpoint, resp.StatusCode)
	}

	return nil
}
