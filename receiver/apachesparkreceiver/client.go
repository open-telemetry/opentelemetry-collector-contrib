// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"
)

const (
	metricsPath      = "/metrics/json"
	applicationsPath = "/api/v1/applications"
)

type client interface {
	Get(path string) ([]byte, error)
	ClusterStats() (*models.ClusterProperties, error)
	Applications() ([]models.Application, error)
	StageStats(appID string) ([]models.Stage, error)
	ExecutorStats(appID string) ([]models.Executor, error)
	JobStats(appID string) ([]models.Job, error)
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

func (c *apacheSparkClient) ClusterStats() (*models.ClusterProperties, error) {
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

func (c *apacheSparkClient) Applications() ([]models.Application, error) {
	body, err := c.Get(applicationsPath)
	if err != nil {
		return nil, err
	}

	var apps []models.Application
	err = json.Unmarshal(body, &apps)
	if err != nil {
		return nil, err
	}

	return apps, nil
}

func (c *apacheSparkClient) StageStats(appID string) ([]models.Stage, error) {
	stagePath := fmt.Sprintf("%s/%s/stages", applicationsPath, appID)
	body, err := c.Get(stagePath)
	if err != nil {
		return nil, err
	}

	var stageStats []models.Stage
	err = json.Unmarshal(body, &stageStats)
	if err != nil {
		return nil, err
	}

	return stageStats, nil
}

func (c *apacheSparkClient) ExecutorStats(appID string) ([]models.Executor, error) {
	executorPath := fmt.Sprintf("%s/%s/executors", applicationsPath, appID)
	body, err := c.Get(executorPath)
	if err != nil {
		return nil, err
	}

	var executorStats []models.Executor
	err = json.Unmarshal(body, &executorStats)
	if err != nil {
		return nil, err
	}

	return executorStats, nil
}

func (c *apacheSparkClient) JobStats(appID string) ([]models.Job, error) {
	jobPath := fmt.Sprintf("%s/%s/jobs", applicationsPath, appID)
	body, err := c.Get(jobPath)
	if err != nil {
		return nil, err
	}

	var jobStats []models.Job
	err = json.Unmarshal(body, &jobStats)
	if err != nil {
		return nil, err
	}

	return jobStats, nil
}

func (c *apacheSparkClient) buildReq(path string) (*http.Request, error) {
	url := c.cfg.Endpoint + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}
