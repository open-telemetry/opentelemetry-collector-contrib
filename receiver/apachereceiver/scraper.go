// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver"

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

type apacheScraper struct {
	settings   component.TelemetrySettings
	cfg        *Config
	httpClient *http.Client
	rb         *metadata.ResourceBuilder
	mb         *metadata.MetricsBuilder
	serverName string
	port       string
}

func newApacheScraper(
	settings receiver.CreateSettings,
	cfg *Config,
	serverName string,
	port string,
) *apacheScraper {
	a := &apacheScraper{
		settings:   settings.TelemetrySettings,
		cfg:        cfg,
		rb:         metadata.NewResourceBuilder(cfg.MetricsBuilderConfig.ResourceAttributes),
		mb:         metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		serverName: serverName,
		port:       port,
	}

	return a
}

func (r *apacheScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(host, r.settings)
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

	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())
	for metricKey, metricValue := range parseStats(stats) {
		switch metricKey {
		case "ServerUptimeSeconds":
			addPartialIfError(errs, r.mb.RecordApacheUptimeDataPoint(now, metricValue))
		case "ConnsTotal":
			addPartialIfError(errs, r.mb.RecordApacheCurrentConnectionsDataPoint(now, metricValue))
		case "BusyWorkers":
			addPartialIfError(errs, r.mb.RecordApacheWorkersDataPoint(now, metricValue, metadata.AttributeWorkersStateBusy))
		case "IdleWorkers":
			addPartialIfError(errs, r.mb.RecordApacheWorkersDataPoint(now, metricValue, metadata.AttributeWorkersStateIdle))
		case "Total Accesses":
			addPartialIfError(errs, r.mb.RecordApacheRequestsDataPoint(now, metricValue))
		case "Total kBytes":
			i, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				errs.AddPartial(1, err)
			} else {
				r.mb.RecordApacheTrafficDataPoint(now, kbytesToBytes(i))
			}
		case "CPUChildrenSystem":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeCPULevelChildren, metadata.AttributeCPUModeSystem),
			)
		case "CPUChildrenUser":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeCPULevelChildren, metadata.AttributeCPUModeUser),
			)
		case "CPUSystem":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeCPULevelSelf, metadata.AttributeCPUModeSystem),
			)
		case "CPUUser":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeCPULevelSelf, metadata.AttributeCPUModeUser),
			)
		case "CPULoad":
			addPartialIfError(errs, r.mb.RecordApacheCPULoadDataPoint(now, metricValue))
		case "Load1":
			addPartialIfError(errs, r.mb.RecordApacheLoad1DataPoint(now, metricValue))
		case "Load5":
			addPartialIfError(errs, r.mb.RecordApacheLoad5DataPoint(now, metricValue))
		case "Load15":
			addPartialIfError(errs, r.mb.RecordApacheLoad15DataPoint(now, metricValue))
		case "Total Duration":
			addPartialIfError(errs, r.mb.RecordApacheRequestTimeDataPoint(now, metricValue))
		case "Scoreboard":
			scoreboardMap := parseScoreboard(metricValue)
			for state, score := range scoreboardMap {
				r.mb.RecordApacheScoreboardDataPoint(now, score, state)
			}
		}
	}

	r.rb.SetApacheServerName(r.serverName)
	r.rb.SetApacheServerPort(r.port)
	return r.mb.Emit(metadata.WithResource(r.rb.Emit())), errs.Combine()
}

func addPartialIfError(errs *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errs.AddPartial(1, err)
	}
}

// GetStats collects metric stats by making a get request at an endpoint.
func (r *apacheScraper) GetStats() (string, error) {
	resp, err := r.httpClient.Get(r.cfg.Endpoint)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
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
