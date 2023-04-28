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

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http" // client defines the basic HTTP client interface.
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

const (
	metricsPath      = "/metrics/json"
	applicationsPath = "/api/v1/applications"
)

type client interface {
	Get(path string) ([]byte, error)
	GetClusterStats() (*models.ClusterProperties, error)
	GetApplications() (*models.Applications, error)
	GetStageStats(appID string) (*models.Stages, error)
	GetExecutorStats(appID string) (*models.Executors, error)
	GetJobStats(appID string) (*models.Jobs, error)
}

var _ client = (*apacheSparkClient)(nil)

type apacheSparkClient struct {
	client *http.Client
	cfg    *Config
	logger *zap.Logger
}

// newApacheSparkClient creates a new client to make requests for the Apache Spark receiver.
func newApacheSparkClient(cfg *Config, host component.Host, settings component.TelemetrySettings) (client, error) {
	client, err := cfg.ToClient(host, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	return &apacheSparkClient{
		client: client,
		cfg:    cfg,
		logger: settings.Logger,
	}, nil
}

// Get issues an authorized Get requests to the specified url.
func (c *apacheSparkClient) Get(path string) ([]byte, error) {
	req, err := c.buildReq(path)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			c.logger.Warn("failed to close response body", zap.Error(err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 400 {
			c.logger.Error("apachespark", zap.Error(err), zap.String("status_code", strconv.Itoa(resp.StatusCode)))
		}
		return nil, fmt.Errorf("request GET %s failed - %q", req.URL.String(), resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body %w", err)
	}

	return body, nil
}

func (c *apacheSparkClient) GetClusterStats() (*models.ClusterProperties, error) {
	body, err := c.Get(metricsPath)
	if err != nil {
		return nil, err
	}

	var clusterStats *models.ClusterProperties
	err = json.Unmarshal(body, &clusterStats)
	if err != nil {
		return nil, err
	}

	return clusterStats, nil
}

func (c *apacheSparkClient) GetApplications() (*models.Applications, error) {
	body, err := c.Get(applicationsPath)
	if err != nil {
		return nil, err
	}

	var apps *models.Applications
	err = json.Unmarshal(body, &apps)
	if err != nil {
		return nil, err
	}

	return apps, nil
}

func (c *apacheSparkClient) GetStageStats(appID string) (*models.Stages, error) {
	stagePath := fmt.Sprintf("/api/v1/applications/%s/stages", appID)
	body, err := c.Get(stagePath)
	if err != nil {
		return nil, err
	}

	var stageStats models.Stages
	err = json.Unmarshal(body, &stageStats)
	if err != nil {
		return nil, err
	}

	return &stageStats, nil
}

func (c *apacheSparkClient) GetExecutorStats(appID string) (*models.Executors, error) {
	executorPath := fmt.Sprintf("/api/v1/applications/%s/executors", appID)
	body, err := c.Get(executorPath)
	if err != nil {
		return nil, err
	}

	var executorStats models.Executors
	err = json.Unmarshal(body, &executorStats)
	if err != nil {
		return nil, err
	}

	return &executorStats, nil
}

func (c *apacheSparkClient) GetJobStats(appID string) (*models.Jobs, error) {
	jobPath := fmt.Sprintf("/api/v1/applications/%s/jobs", appID)
	body, err := c.Get(jobPath)
	if err != nil {
		return nil, err
	}

	var jobStats models.Jobs
	err = json.Unmarshal(body, &jobStats)
	if err != nil {
		return nil, err
	}

	return &jobStats, nil
}

func (c *apacheSparkClient) buildReq(path string) (*http.Request, error) {
	url := c.cfg.Endpoint + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}
