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
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

type apacheScraper struct {
	settings   component.TelemetrySettings
	cfg        *Config
	httpClient *http.Client
	mb         *metadata.MetricsBuilder
	serverName string
	port       string
}

func newApacheScraper(
	settings receiver.Settings,
	cfg *Config,
	serverName string,
	port string,
) *apacheScraper {
	a := &apacheScraper{
		settings:   settings.TelemetrySettings,
		cfg:        cfg,
		mb:         metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		serverName: serverName,
		port:       port,
	}

	return a
}

func (r *apacheScraper) start(ctx context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(ctx, host.GetExtensions(), r.settings)
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
			addPartialIfError(errs, r.mb.RecordApacheConnectionActiveDataPoint(now, metricValue))
		case "ConnsAsyncWriting":
			addPartialIfError(errs, r.mb.RecordApacheConnectionsDataPoint(now, metricValue, metadata.AttributeApacheConnectionStateWriting))
		case "ConnsAsyncKeepAlive":
			addPartialIfError(errs, r.mb.RecordApacheConnectionsDataPoint(now, metricValue, metadata.AttributeApacheConnectionStateKeepalive))
		case "ConnsAsyncClosing":
			addPartialIfError(errs, r.mb.RecordApacheConnectionsDataPoint(now, metricValue, metadata.AttributeApacheConnectionStateClosing))
		case "BusyWorkers":
			addPartialIfError(errs, r.mb.RecordApacheWorkerActiveDataPoint(now, metricValue))
		case "IdleWorkers":
			addPartialIfError(errs, r.mb.RecordApacheWorkerIdleDataPoint(now, metricValue))
		case "Total Accesses":
			addPartialIfError(errs, r.mb.RecordApacheRequestCountDataPoint(now, metricValue))
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
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeApacheProcessLevelChildren, metadata.AttributeCPUModeSystem),
			)
		case "CPUChildrenUser":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeApacheProcessLevelChildren, metadata.AttributeCPUModeUser),
			)
		case "CPUSystem":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeApacheProcessLevelSelf, metadata.AttributeCPUModeSystem),
			)
		case "CPUUser":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeApacheProcessLevelSelf, metadata.AttributeCPUModeUser),
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
				r.mb.RecordApacheWorkersDataPoint(now, score, state)
			}
		}
	}

	rb := r.mb.NewResourceBuilder()
	rb.SetApacheServerName(r.serverName)
	rb.SetApacheServerPort(r.port)
	return r.mb.Emit(metadata.WithResource(rb.Emit())), errs.Combine()
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

	for field := range strings.SplitSeq(resp, "\n") {
		key, value, found := strings.Cut(field, ": ")
		if !found {
			continue
		}
		metrics[key] = value
	}
	return metrics
}

type scoreboardCountsByLabel map[metadata.AttributeApacheWorkerState]int64

// parseScoreboard quantifies the symbolic mapping of the scoreboard.
func parseScoreboard(values string) scoreboardCountsByLabel {
	scoreboard := scoreboardCountsByLabel{
		metadata.AttributeApacheWorkerStateWaiting:     0,
		metadata.AttributeApacheWorkerStateStarting:    0,
		metadata.AttributeApacheWorkerStateReading:     0,
		metadata.AttributeApacheWorkerStateSending:     0,
		metadata.AttributeApacheWorkerStateKeepalive:   0,
		metadata.AttributeApacheWorkerStateDnslookup:   0,
		metadata.AttributeApacheWorkerStateClosing:     0,
		metadata.AttributeApacheWorkerStateLogging:     0,
		metadata.AttributeApacheWorkerStateFinishing:   0,
		metadata.AttributeApacheWorkerStateIdleCleanup: 0,
		metadata.AttributeApacheWorkerStateOpen:        0,
	}

	for _, char := range values {
		switch string(char) {
		case "_":
			scoreboard[metadata.AttributeApacheWorkerStateWaiting]++
		case "S":
			scoreboard[metadata.AttributeApacheWorkerStateStarting]++
		case "R":
			scoreboard[metadata.AttributeApacheWorkerStateReading]++
		case "W":
			scoreboard[metadata.AttributeApacheWorkerStateSending]++
		case "K":
			scoreboard[metadata.AttributeApacheWorkerStateKeepalive]++
		case "D":
			scoreboard[metadata.AttributeApacheWorkerStateDnslookup]++
		case "C":
			scoreboard[metadata.AttributeApacheWorkerStateClosing]++
		case "L":
			scoreboard[metadata.AttributeApacheWorkerStateLogging]++
		case "G":
			scoreboard[metadata.AttributeApacheWorkerStateFinishing]++
		case "I":
			scoreboard[metadata.AttributeApacheWorkerStateIdleCleanup]++
		case ".":
			scoreboard[metadata.AttributeApacheWorkerStateOpen]++
		default:
			scoreboard[metadata.AttributeApacheWorkerStateUnknown]++
		}
	}
	return scoreboard
}

// kbytesToBytes converts 1 Kibibyte to 1024 bytes.
func kbytesToBytes(i int64) int64 {
	return 1024 * i
}
