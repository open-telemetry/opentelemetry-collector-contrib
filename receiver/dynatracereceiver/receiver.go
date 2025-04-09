// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatracereceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Receiver struct {
	Config     *Config
	NextMetric consumer.Metrics
	ticker     *time.Ticker
	stopChan   chan struct{}
	httpClient *http.Client
}

type DynatraceResponse struct {
	TotalCount  int                   `json:"totalCount"`
	NextPageKey string                `json:"nextPageKey"`
	Resolution  string                `json:"resolution"`
	Result      []DynatraceMetricData `json:"result"`
}

type DynatraceMetricData struct {
	MetricID string         `json:"metricId"`
	Data     []MetricValues `json:"data"`
}

type MetricValues struct {
	Timestamps   []int64           `json:"timestamps"`
	Values       []float64         `json:"values"`
	Dimensions   []string          `json:"dimensions"`
	DimensionMap map[string]string `json:"dimensionMap"`
}

// start polling from Dynatrace.
func (r *Receiver) Start(ctx context.Context, host component.Host) error { // revive:disable-line:unused-parameter
	fmt.Println("Dynatrace Receiver started with config:", r.Config)

	r.ticker = time.NewTicker(r.Config.PollInterval)
	r.stopChan = make(chan struct{})

	if r.httpClient == nil {
		r.httpClient = &http.Client{
			Timeout: r.Config.HTTPTimeout,
		}
	}

	go func() {
		for {
			select {
			case <-r.ticker.C:
				metrics, err := r.pullDynatraceMetrics(ctx, r.Config)
				if err != nil {
					fmt.Println("Error pulling metrics:", err)
				}

				md := convertToMetricData(metrics)
				if err := r.NextMetric.ConsumeMetrics(ctx, md); err != nil {
					fmt.Println("Error consuming metrics:", err)
				}

			case <-r.stopChan:
				fmt.Println("Stopping Dynatrace Receiver polling loop.")
				return
			}
		}
	}()

	return nil
}

func (r *Receiver) Shutdown(_ context.Context) error {
	fmt.Println("Dynatrace Receiver shutting down.")
	r.ticker.Stop()
	close(r.stopChan)
	return nil
}

func (r *Receiver) pullDynatraceMetrics(ctx context.Context, cfg *Config) ([]DynatraceMetricData, error) {
	var metrics []DynatraceMetricData
	var err error

	for i := 0; i < cfg.MaxRetries; i++ {
		metrics, err = r.fetchAllDynatraceMetrics(ctx, cfg)
		if err == nil {
			return metrics, nil
		}
		fmt.Printf("Attempt %d failed: %v\n", i+1, err)
		time.Sleep(time.Second * time.Duration(i+1)) // simple backoff
	}
	return nil, fmt.Errorf("all retries failed: %w", err)
}

func (r *Receiver) fetchAllDynatraceMetrics(ctx context.Context, cfg *Config) ([]DynatraceMetricData, error) {
	url := createMetricsQuery(cfg)

	ctx, cancel := context.WithTimeout(ctx, cfg.HTTPTimeout)
	defer cancel()

	resp, err := r.makeHttPRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var dtResponse DynatraceResponse
	if err := json.Unmarshal(body, &dtResponse); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	return dtResponse.Result, nil
}

func createMetricsQuery(cfg *Config) string {
	metricSelector := strings.Join(cfg.MetricSelectors, ",")
	url := fmt.Sprintf("%s?metricSelector=%s&resolution=%s&from=%s&to=%s", cfg.APIEndpoint, metricSelector, cfg.Resolution, cfg.From, cfg.To)

	fmt.Println("Fetching data from:", url)
	return url
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body failed: %w", err)
	}
	return body, nil
}

func (r *Receiver) makeHttPRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Api-Token "+r.Config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("dynatrace returned non-2xx status: %d", resp.StatusCode)
	}

	return resp, nil
}

func convertToMetricData(metrics []DynatraceMetricData) pmetric.Metrics {
	md := pmetric.NewMetrics()

	for _, metric := range metrics {
		for _, data := range metric.Data {
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			m := sm.Metrics().AppendEmpty()

			m.SetName(metric.MetricID)
			gauge := m.SetEmptyGauge()

			for i, timestamp := range data.Timestamps {
				if i < len(data.Values) {
					dp := gauge.DataPoints().AppendEmpty()
					dp.SetTimestamp(pcommon.Timestamp(timestamp * 1e6))
					dp.SetDoubleValue(data.Values[i])

					for key, val := range data.DimensionMap {
						dp.Attributes().PutStr(key, val)
					}
				}
			}
		}
	}
	return md
}
