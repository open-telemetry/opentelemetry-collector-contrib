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

package apachereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

type apacheScraper struct {
	settings   component.TelemetrySettings
	cfg        *Config
	httpClient *http.Client
	mb         *metadata.MetricsBuilder
}

func newApacheScraper(
	settings component.TelemetrySettings,
	cfg *Config,
) *apacheScraper {
	return &apacheScraper{
		settings: settings,
		cfg:      cfg,
		mb:       metadata.NewMetricsBuilder(cfg.Metrics),
	}
}

func (r *apacheScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(host.GetExtensions(), r.settings)
	if err != nil {
		return err
	}
	r.httpClient = httpClient
	return nil
}

func (r *apacheScraper) scrape(context.Context) (pdata.Metrics, error) {
	if r.httpClient == nil {
		return pdata.Metrics{}, errors.New("failed to connect to Apache HTTPd")
	}

	stats, err := r.GetStats()
	if err != nil {
		r.settings.Logger.Error("failed to fetch Apache Httpd stats", zap.Error(err))
		return pdata.Metrics{}, err
	}

	now := pdata.NewTimestampFromTime(time.Now())
	for metricKey, metricValue := range parseStats(stats) {
		switch metricKey {
		case "ServerUptimeSeconds":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				r.mb.RecordApacheUptimeDataPoint(now, i, r.cfg.serverName)
			}
		case "ConnsTotal":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				r.mb.RecordApacheCurrentConnectionsDataPoint(now, i, r.cfg.serverName)
			}
		case "BusyWorkers":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				r.mb.RecordApacheWorkersDataPoint(now, i, r.cfg.serverName, "busy")
			}
		case "IdleWorkers":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				r.mb.RecordApacheWorkersDataPoint(now, i, r.cfg.serverName, "idle")
			}
		case "Total Accesses":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				r.mb.RecordApacheRequestsDataPoint(now, i, r.cfg.serverName)
			}
		case "Total kBytes":
			if i, ok := r.parseInt(metricKey, metricValue); ok {
				r.mb.RecordApacheTrafficDataPoint(now, kbytesToBytes(i), r.cfg.serverName)
			}
		case "Scoreboard":
			scoreboardMap := parseScoreboard(metricValue)
			for state, score := range scoreboardMap {
				r.mb.RecordApacheScoreboardDataPoint(now, score, r.cfg.serverName, state)
			}
		}
	}

	return r.mb.Emit(), nil
}

// GetStats collects metric stats by making a get request at an endpoint.
func (r *apacheScraper) GetStats() (string, error) {
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
func (r *apacheScraper) parseInt(key, value string) (int64, bool) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		r.logInvalid("int", key, value)
		return 0, false
	}
	return i, true
}

func (r *apacheScraper) logInvalid(expectedType, key, value string) {
	r.settings.Logger.Info(
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
