// Copyright  OpenTelemetry Authors
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

package httpdreceiver

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpdreceiver/internal/metadata"
)

type httpdScraper struct {
	logger     *zap.Logger
	cfg        *Config
	httpClient *http.Client
}

func newHttpdScraper(
	logger *zap.Logger,
	cfg *Config,
) *httpdScraper {
	return &httpdScraper{
		logger: logger,
		cfg:    cfg,
	}
}

func (r *httpdScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(host.GetExtensions())
	if err != nil {
		return err
	}
	r.httpClient = httpClient
	return nil
}

func initMetric(ms pdata.MetricSlice, mi metadata.MetricIntf) pdata.Metric {
	m := ms.AppendEmpty()
	mi.Init(m)
	return m
}

func addToIntMetric(metric pdata.NumberDataPointSlice, labels pdata.AttributeMap, value int64, ts pdata.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(ts)
	dataPoint.SetIntVal(value)
	if labels.Len() > 0 {
		labels.CopyTo(dataPoint.Attributes())
	}
}

func (r *httpdScraper) scrape(context.Context) (pdata.ResourceMetricsSlice, error) {
	if r.httpClient == nil {
		return pdata.ResourceMetricsSlice{}, errors.New("failed to connect to Apache HTTPd")
	}

	stats, err := r.GetStats()
	if err != nil {
		r.logger.Error("failed to fetch HTTPd stats", zap.Error(err))
		return pdata.ResourceMetricsSlice{}, err
	}

	rms := pdata.NewResourceMetricsSlice()
	ilm := rms.AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otel/httpd")
	now := pdata.NewTimestampFromTime(time.Now())

	uptime := initMetric(ilm.Metrics(), metadata.M.HttpdUptime).Sum().DataPoints()
	connections := initMetric(ilm.Metrics(), metadata.M.HttpdCurrentConnections).Sum().DataPoints()
	workers := initMetric(ilm.Metrics(), metadata.M.HttpdWorkers).Sum().DataPoints()
	requests := initMetric(ilm.Metrics(), metadata.M.HttpdRequests).Sum().DataPoints()
	traffic := initMetric(ilm.Metrics(), metadata.M.HttpdTraffic).Sum().DataPoints()
	scoreboard := initMetric(ilm.Metrics(), metadata.M.HttpdScoreboard).Sum().DataPoints()

	for metricKey, metricValue := range parseStats(stats) {
		labels := pdata.NewAttributeMap()
		labels.Insert(metadata.L.ServerName, pdata.NewAttributeValueString(r.cfg.serverName))

		switch metricKey {
		case "ServerUptimeSeconds":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				addToIntMetric(uptime, labels, i, now)
			}
		case "ConnsTotal":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				addToIntMetric(connections, labels, i, now)
			}
		case "BusyWorkers":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				labels.Insert(metadata.L.WorkersState, pdata.NewAttributeValueString("busy"))
				addToIntMetric(workers, labels, i, now)
			}
		case "IdleWorkers":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				labels.Insert(metadata.L.WorkersState, pdata.NewAttributeValueString("idle"))
				addToIntMetric(workers, labels, i, now)
			}
		case "Total Accesses":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				addToIntMetric(requests, labels, i, now)
			}
		case "Total kBytes":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				bytes := kbytesToBytes(i)
				addToIntMetric(traffic, labels, bytes, now)
			}
		case "Scoreboard":
			scoreboardMap := parseScoreboard(metricValue)
			for identifier, score := range scoreboardMap {
				labels.Upsert(metadata.L.ScoreboardState, pdata.NewAttributeValueString(identifier))
				addToIntMetric(scoreboard, labels, score, now)
			}
		}
	}

	return rms, nil
}

// GetStats collects metric stats by making a get request at an endpoint.
func (r *httpdScraper) GetStats() (string, error) {
	resp, err := r.httpClient.Get(r.cfg.Endpoint)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// parseStats converts a response body key:values into a map.
func parseStats(resp string) map[string]string {
	metrics := make(map[string]string)

	fields := strings.Split(resp, "\n")
	for _, field := range fields {
		index := strings.Index(field, ": ")
		if index == -1 {
			continue
		}
		metrics[field[:index]] = field[index+2:]
	}
	return metrics
}

// parseInt converts string to int64.
func (r *httpdScraper) parseInt(key, value string) (int64, bool) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		r.logInvalid("int", key, value)
		return 0, false
	}
	return i, true
}

func (r *httpdScraper) logInvalid(expectedType, key, value string) {
	r.logger.Info(
		"invalid value",
		zap.String("expectedType", expectedType),
		zap.String("key", key),
		zap.String("value", value),
	)
}

type scoreboardCountsByLabel map[string]int64

// parseScoreboard quantifies the symbolic mapping of the scoreboard.
func parseScoreboard(values string) scoreboardCountsByLabel {
	scoreboard := scoreboardCountsByLabel{
		"waiting":      0,
		"starting":     0,
		"reading":      0,
		"sending":      0,
		"keepalive":    0,
		"dnslookup":    0,
		"closing":      0,
		"logging":      0,
		"finishing":    0,
		"idle_cleanup": 0,
		"open":         0,
	}

	for _, char := range values {
		switch string(char) {
		case "_":
			scoreboard["waiting"]++
		case "S":
			scoreboard["starting"]++
		case "R":
			scoreboard["reading"]++
		case "W":
			scoreboard["sending"]++
		case "K":
			scoreboard["keepalive"]++
		case "D":
			scoreboard["dnslookup"]++
		case "C":
			scoreboard["closing"]++
		case "L":
			scoreboard["logging"]++
		case "G":
			scoreboard["finishing"]++
		case "I":
			scoreboard["idle_cleanup"]++
		case ".":
			scoreboard["open"]++
		default:
			scoreboard["unknown"]++
		}
	}
	return scoreboard
}

// kbytesToBytes converts 1 Kibibyte to 1024 bytes.
func kbytesToBytes(i int64) int64 {
	return 1024 * i
}
