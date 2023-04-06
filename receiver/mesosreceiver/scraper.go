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

package mesosreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mesosreceiver/internal/metadata"
)

type mesosScraper struct {
	settings   component.TelemetrySettings
	cfg        *Config
	httpClient *http.Client
	mb         *metadata.MetricsBuilder
	serverName string
	port       string
}

func newMesosScraper(
	settings receiver.CreateSettings,
	cfg *Config,
	serverName string,
	port string,
) *mesosScraper {
	m := &mesosScraper{
		settings:   settings.TelemetrySettings,
		cfg:        cfg,
		mb:         metadata.NewMetricsBuilder(metadata.NewMetricsBuilderConfig(cfg.Metrics, metadata.DefaultResourceAttributesSettings()), settings),
		serverName: serverName,
		port:       port,
	}
	return m
}

// Makes a get request to the endpoint to collect endpoint provided stats.
func (r *mesosScraper) GetStats() ([]byte, error) {
	resp, err := r.httpClient.Get(r.cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// parseStats converts a response body key:values into a map.
func parseStats(resp []byte) map[string]string {
	var metrics map[string]interface{}
	json.Unmarshal(resp, &metrics)

	result := make(map[string]string)
	for k, v := range metrics {
		switch val := v.(type) {
		case string:
			result[k] = val
		case float64:
			result[k] = strconv.FormatFloat(val, 'f', -1, 64)
		case bool:
			result[k] = strconv.FormatBool(val)
		case nil:
			result[k] = ""
		default:
			result[k] = fmt.Sprintf("%v", val)
		}
	}
	return result
}

func addPartialIfError(errs *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errs.AddPartial(1, err)
	}
}

func (r *mesosScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(host, r.settings)
	if err != nil {
		return err
	}

	r.httpClient = httpClient
	return nil
}

func (r *mesosScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if r.httpClient == nil {
		return pmetric.Metrics{}, errors.New("failed to connect to Mesos")
	}
	stats, err := r.GetStats()
	if err != nil {
		r.settings.Logger.Error("failed to fetch Mesos Stats", zap.Error(err))
		return pmetric.Metrics{}, errors.New("failed to fetch Mesos Stats")
	}
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	for metricKey, metricValue := range parseStats(stats) {
		switch metricKey {
		case "master/cpus_percent":
			addPartialIfError(errs, r.mb.RecordMesosCPUUtilizationDataPoint(now, metricValue))
		case "master/gpus_percent":
			addPartialIfError(errs, r.mb.RecordMesosGpuUtilizationDataPoint(now, metricValue))
		case "master/mem_percent":
			addPartialIfError(errs, r.mb.RecordMesosMemUtilizationDataPoint(now, metricValue))
		case "master/mem_total":
			addPartialIfError(errs, r.mb.RecordMesosMemLimitDataPoint(now, convMegabytesToBytes(metricValue)))
		case "master/uptime_secs":
			addPartialIfError(errs, r.mb.RecordMesosUptimeDataPoint(now, metricValue))
		case "system/load_15min":
			addPartialIfError(errs, r.mb.RecordSystemLoad15mDataPoint(now, metricValue))
		case "master/slaves_active":
			addPartialIfError(errs, r.mb.RecordMesosSlavesActiveCountDataPoint(now, metricValue))
		case "master/slaves_connected":
			addPartialIfError(errs, r.mb.RecordMesosSlavesConnectedCountDataPoint(now, metricValue))
		case "master/slaves_disconnected":
			addPartialIfError(errs, r.mb.RecordMesosSlavesDisconnectedCountDataPoint(now, metricValue))
		case "master/slaves_inactive":
			addPartialIfError(errs, r.mb.RecordMesosSlavesInactiveCountDataPoint(now, metricValue))
		case "master/tasks_failed":
			addPartialIfError(errs, r.mb.RecordMesosTasksFailedCountDataPoint(now, metricValue))
		case "master/tasks_finished":
			addPartialIfError(errs, r.mb.RecordMesosTasksFinishedCountDataPoint(now, metricValue))
		}
	}
	return r.mb.Emit(metadata.WithMesosServerName(r.serverName), metadata.WithMesosServerPort(r.port)), errs.Combine()
}

func convMegabytesToBytes(mVal string) string {
	mem, _ := strconv.ParseFloat(mVal, 64)
	return fmt.Sprintf("%.0f", (1000000.0 * mem))
}
