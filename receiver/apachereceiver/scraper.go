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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
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
	settings component.ReceiverCreateSettings,
	cfg *Config,
) *apacheScraper {
	return &apacheScraper{
		settings: settings.TelemetrySettings,
		cfg:      cfg,
		mb:       metadata.NewMetricsBuilder(cfg.Metrics, settings.BuildInfo),
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

func (r *apacheScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if r.httpClient == nil {
		return pmetric.Metrics{}, errors.New("failed to connect to Apache HTTPd")
	}

	stats, err := r.GetStats()
	if err != nil {
		r.settings.Logger.Error("failed to fetch Apache Httpd stats", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	var errors scrapererror.ScrapeErrors
	now := pcommon.NewTimestampFromTime(time.Now())
	for metricKey, metricValue := range parseStats(stats) {
		switch metricKey {
		case "ServerUptimeSeconds":
			addPartialIfError(errors, r.mb.RecordApacheUptimeDataPoint(now, metricValue, r.cfg.serverName))
		case "ConnsTotal":
			addPartialIfError(errors, r.mb.RecordApacheCurrentConnectionsDataPoint(now, metricValue, r.cfg.serverName))
		case "BusyWorkers":
			addPartialIfError(errors, r.mb.RecordApacheWorkersDataPoint(now, metricValue, r.cfg.serverName,
				metadata.AttributeWorkersStateBusy))
		case "IdleWorkers":
			addPartialIfError(errors, r.mb.RecordApacheWorkersDataPoint(now, metricValue, r.cfg.serverName,
				metadata.AttributeWorkersStateIdle))
		case "Total Accesses":
			addPartialIfError(errors, r.mb.RecordApacheRequestsDataPoint(now, metricValue, r.cfg.serverName))
		case "Total kBytes":
			i, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				errors.AddPartial(1, err)
			} else {
				r.mb.RecordApacheTrafficDataPoint(now, kbytesToBytes(i), r.cfg.serverName)
			}
		case "Scoreboard":
			scoreboardMap := parseScoreboard(metricValue)
			for state, score := range scoreboardMap {
				r.mb.RecordApacheScoreboardDataPoint(now, score, r.cfg.serverName, state)
			}
		}
	}

	return r.mb.Emit(), errors.Combine()
}

func addPartialIfError(errors scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errors.AddPartial(1, err)
	}
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

type scoreboardCountsByLabel map[metadata.AttributeScoreboardState]int64

// parseScoreboard quantifies the symbolic mapping of the scoreboard.
func parseScoreboard(values string) scoreboardCountsByLabel {
	scoreboard := scoreboardCountsByLabel{
		metadata.AttributeScoreboardStateWaiting:     0,
		metadata.AttributeScoreboardStateStarting:    0,
		metadata.AttributeScoreboardStateReading:     0,
		metadata.AttributeScoreboardStateSending:     0,
		metadata.AttributeScoreboardStateKeepalive:   0,
		metadata.AttributeScoreboardStateDnslookup:   0,
		metadata.AttributeScoreboardStateClosing:     0,
		metadata.AttributeScoreboardStateLogging:     0,
		metadata.AttributeScoreboardStateFinishing:   0,
		metadata.AttributeScoreboardStateIdleCleanup: 0,
		metadata.AttributeScoreboardStateOpen:        0,
	}

	for _, char := range values {
		switch string(char) {
		case "_":
			scoreboard[metadata.AttributeScoreboardStateWaiting]++
		case "S":
			scoreboard[metadata.AttributeScoreboardStateStarting]++
		case "R":
			scoreboard[metadata.AttributeScoreboardStateReading]++
		case "W":
			scoreboard[metadata.AttributeScoreboardStateSending]++
		case "K":
			scoreboard[metadata.AttributeScoreboardStateKeepalive]++
		case "D":
			scoreboard[metadata.AttributeScoreboardStateDnslookup]++
		case "C":
			scoreboard[metadata.AttributeScoreboardStateClosing]++
		case "L":
			scoreboard[metadata.AttributeScoreboardStateLogging]++
		case "G":
			scoreboard[metadata.AttributeScoreboardStateFinishing]++
		case "I":
			scoreboard[metadata.AttributeScoreboardStateIdleCleanup]++
		case ".":
			scoreboard[metadata.AttributeScoreboardStateOpen]++
		default:
			scoreboard[metadata.AttributeScoreboardStateUnknown]++
		}
	}
	return scoreboard
}

// kbytesToBytes converts 1 Kibibyte to 1024 bytes.
func kbytesToBytes(i int64) int64 {
	return 1024 * i
}
