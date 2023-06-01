// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
