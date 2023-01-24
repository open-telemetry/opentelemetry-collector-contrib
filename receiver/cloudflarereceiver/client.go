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

// Package cloudflarereceiver collects cloudflare logs into OTLP format.
package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudflare/cloudflare-go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/models"
)

type client interface {
	MakeRequest(ctx context.Context, startTime string, endTime string) ([]*models.Log, error)
	BuildEndpoint(startTime string, endTime string) string
}

var _ client = (*cloudflareClient)(nil)

var defaultBaseURL = "https://api.cloudflare.com/client/v4"

type cloudflareClient struct {
	api      *cloudflare.API
	cfg      *Config
	endpoint string
}

func newCloudflareClient(cfg *Config, baseURL string) (client, error) {
	var api *cloudflare.API
	var err error
	switch {
	case cfg.Auth.XAuthEmail != "" && cfg.Auth.XAuthKey != "":
		api, err = cloudflare.New(cfg.Auth.XAuthKey, cfg.Auth.XAuthEmail)
	case cfg.Auth.APIToken != "":
		api, err = cloudflare.NewWithAPIToken(cfg.Auth.APIToken)
	default:
		return nil, errInvalidAuthenticationConfigured
	}
	api.BaseURL = baseURL

	if err != nil {
		return nil, err
	}

	return &cloudflareClient{
		api:      api,
		cfg:      cfg,
		endpoint: defaultBaseURL,
	}, nil
}

func (c *cloudflareClient) MakeRequest(ctx context.Context, startTime string, endTime string) ([]*models.Log, error) {
	var parameters interface{}
	endpoint := c.BuildEndpoint(startTime, endTime)

	body, err := c.api.Raw(ctx, http.MethodGet, endpoint, parameters, http.Header{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Cloudflare logs: %w", err)
	}

	var logs []*models.Log
	err = json.Unmarshal(body, &logs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return logs, nil
}

func (c *cloudflareClient) BuildEndpoint(startTime string, endTime string) string {
	url := fmt.Sprintf("/zones/%s/logs/received", c.cfg.Zone)

	fieldsList := strings.Join(c.cfg.Logs.Fields, ",")
	endpoint := fmt.Sprintf("%s?start=%s&end=%s&fields=%s&sample=%f", url, startTime, endTime, fieldsList, c.cfg.Logs.Sample)
	if c.cfg.Logs.Count != 0 {
		endpoint += fmt.Sprintf("&count=%v", c.cfg.Logs.Count)
	}

	return endpoint
}
