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

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	idleConnTimeout      = 30 * time.Second
	tlsHandshakeTimeout  = 10 * time.Second
	dialerTimeout        = 30 * time.Second
	dialerKeepAlive      = 30 * time.Second
	defaultSplunkAppName = "OpenTelemetry Collector Contrib"
)

type splunkExporter struct {
	pushMetricsData func(ctx context.Context, md pmetric.Metrics) error
	pushTraceData   func(ctx context.Context, td ptrace.Traces) error
	pushLogData     func(ctx context.Context, td plog.Logs) error
	stop            func(ctx context.Context) (err error)
	start           func(ctx context.Context, host component.Host) (err error)
}

type exporterOptions struct {
	url   *url.URL
	token configopaque.String
}

// createExporter returns a new Splunk exporter.
func createExporter(
	config *Config,
	logger *zap.Logger,
	buildinfo *component.BuildInfo,
) (*splunkExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	if config.SplunkAppName == "" {
		config.SplunkAppName = defaultSplunkAppName
	}

	if config.SplunkAppVersion == "" {
		config.SplunkAppVersion = buildinfo.Version
	}

	options, err := config.getOptionsFromConfig()
	if err != nil {
		return nil, err
	}

	httpClient, err := buildHTTPClient(config)
	if err != nil {
		return nil, err
	}

	client := buildClient(options, config, httpClient, logger)

	if config.HecHealthCheckEnabled {
		healthCheckURL := options.url
		healthCheckURL.Path = config.HealthPath
		if err := checkHecHealth(httpClient, healthCheckURL); err != nil {
			return nil, fmt.Errorf("health check failed: %w", err)
		}
	}

	return &splunkExporter{
		pushMetricsData: client.pushMetricsData,
		pushTraceData:   client.pushTraceData,
		pushLogData:     client.pushLogData,
		stop:            client.stop,
		start:           client.start,
	}, nil
}

func checkHecHealth(client *http.Client, healthCheckURL *url.URL) error {

	req, err := http.NewRequest("GET", healthCheckURL.String(), nil)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}

	return nil
}

func buildClient(options *exporterOptions, config *Config, httpClient *http.Client, logger *zap.Logger) *client {
	return &client{
		logger:    logger,
		config:    config,
		hecWorker: &defaultHecWorker{options.url, httpClient, buildHTTPHeaders(config)},
	}
}

func buildHTTPClient(config *Config) (*http.Client, error) {
	tlsCfg, err := config.TLSSetting.LoadTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve TLS config for Splunk HEC Exporter: %w", err)
	}
	return &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialerTimeout,
				KeepAlive: dialerKeepAlive,
			}).DialContext,
			MaxIdleConns:        int(config.MaxConnections),
			MaxIdleConnsPerHost: int(config.MaxConnections),
			IdleConnTimeout:     idleConnTimeout,
			TLSHandshakeTimeout: tlsHandshakeTimeout,
			TLSClientConfig:     tlsCfg,
		}}, nil
}
func buildHTTPHeaders(config *Config) map[string]string {
	return map[string]string{
		"Connection":           "keep-alive",
		"Content-Type":         "application/json",
		"User-Agent":           config.SplunkAppName + "/" + config.SplunkAppVersion,
		"Authorization":        splunk.HECTokenHeader + " " + string(config.Token),
		"__splunk_app_name":    config.SplunkAppName,
		"__splunk_app_version": config.SplunkAppVersion,
	}
}
