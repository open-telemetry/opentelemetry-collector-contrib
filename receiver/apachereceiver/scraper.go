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

const (
	migrationGuideURL = "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/apachereceiver/README.md#metric-and-attribute-name-migration"

	// Attribute keys used by the apache.cpu.time metric. The metric name is
	// unchanged between the old and new formats, but its attribute keys are
	// renamed, so the new-format keys are applied as a post-processing step.
	cpuTimeOldLevelKey = "level"
	cpuTimeOldModeKey  = "mode"
	cpuTimeNewLevelKey = "apache.process.level"
	cpuTimeNewModeKey  = "cpu.mode"

	cpuTimeMetricName = "apache.cpu.time"
)

type apacheScraper struct {
	settings   component.TelemetrySettings
	cfg        *Config
	httpClient *http.Client
	mb         *metadata.MetricsBuilder
	serverName string
	port       string

	// emitOld reports whether the original metric and attribute names should be
	// emitted. emitNew reports whether the new, consistently named metrics and
	// attributes should be emitted. Both are derived from feature gates; by
	// default only the old format is emitted.
	emitOld bool
	emitNew bool
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
		emitOld:    !metadata.ReceiverApacheDisableOldFormatMetricsFeatureGate.IsEnabled(),
		emitNew:    metadata.ReceiverApacheEnableNewFormatMetricsFeatureGate.IsEnabled(),
	}

	return a
}

func (r *apacheScraper) start(ctx context.Context, host component.Host) error {
	if !r.emitNew {
		r.settings.Logger.Warn(
			"The apache receiver's metric and attribute names will change in a future release to be more consistent. "+
				"The current (original) names are still emitted by default. To preview and migrate to the new names, "+
				"enable the receiver.apache.enableNewFormatMetrics feature gate. Review any OTTL statements, dashboards, "+
				"alerts and routing that reference the current names before migrating.",
			zap.String("documentation", migrationGuideURL),
		)
	}

	httpClient, err := r.cfg.ClientConfig.ToClient(ctx, host.GetExtensions(), r.settings)
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
			if r.emitOld {
				addPartialIfError(errs, r.mb.RecordApacheCurrentConnectionsDataPoint(now, metricValue))
			}
			if r.emitNew {
				addPartialIfError(errs, r.mb.RecordApacheConnectionActiveDataPoint(now, metricValue))
			}
		case "ConnsAsyncWriting":
			r.recordConnectionsAsync(errs, now, metricValue, metadata.AttributeConnectionStateWriting, metadata.AttributeApacheConnectionStateWriting)
		case "ConnsAsyncKeepAlive":
			r.recordConnectionsAsync(errs, now, metricValue, metadata.AttributeConnectionStateKeepalive, metadata.AttributeApacheConnectionStateKeepalive)
		case "ConnsAsyncClosing":
			r.recordConnectionsAsync(errs, now, metricValue, metadata.AttributeConnectionStateClosing, metadata.AttributeApacheConnectionStateClosing)
		case "BusyWorkers":
			if r.emitOld {
				addPartialIfError(errs, r.mb.RecordApacheWorkersDataPoint(now, metricValue, metadata.AttributeWorkersStateBusy))
			}
			if r.emitNew {
				addPartialIfError(errs, r.mb.RecordApacheWorkerActiveDataPoint(now, metricValue))
			}
		case "IdleWorkers":
			if r.emitOld {
				addPartialIfError(errs, r.mb.RecordApacheWorkersDataPoint(now, metricValue, metadata.AttributeWorkersStateIdle))
			}
			if r.emitNew {
				addPartialIfError(errs, r.mb.RecordApacheWorkerIdleDataPoint(now, metricValue))
			}
		case "Total Accesses":
			if r.emitOld {
				addPartialIfError(errs, r.mb.RecordApacheRequestsDataPoint(now, metricValue))
			}
			if r.emitNew {
				addPartialIfError(errs, r.mb.RecordApacheRequestCountDataPoint(now, metricValue))
			}
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
		case "ReqPerSec":
			addPartialIfError(errs, r.mb.RecordApacheRequestRateDataPoint(now, metricValue))
		case "BytesPerSec":
			addPartialIfError(errs, r.mb.RecordApacheTrafficRateDataPoint(now, metricValue))
		case "Scoreboard":
			r.mb.RecordApacheWorkerLimitDataPoint(now, int64(len(metricValue)))
			scoreboardMap := parseScoreboard(metricValue)
			for state, score := range scoreboardMap {
				if r.emitOld {
					r.mb.RecordApacheScoreboardDataPoint(now, score, metadata.MapAttributeScoreboardState[state])
				}
				if r.emitNew {
					r.mb.RecordApacheWorkerStatusDataPoint(now, score, metadata.MapAttributeApacheWorkerState[state])
				}
			}
		}
	}

	rb := r.mb.NewResourceBuilder()
	rb.SetApacheServerName(r.serverName)
	rb.SetApacheServerPort(r.port)
	metrics := r.mb.Emit(metadata.WithResource(rb.Emit()))
	r.applyCPUTimeNewFormat(metrics)
	return metrics, errs.Combine()
}

// recordConnectionsAsync records the asynchronous connection count under the old
// metric/attribute names, the new ones, or both, depending on the feature gates.
func (r *apacheScraper) recordConnectionsAsync(
	errs *scrapererror.ScrapeErrors,
	now pcommon.Timestamp,
	value string,
	oldState metadata.AttributeConnectionState,
	newState metadata.AttributeApacheConnectionState,
) {
	if r.emitOld {
		addPartialIfError(errs, r.mb.RecordApacheConnectionsAsyncDataPoint(now, value, oldState))
	}
	if r.emitNew {
		addPartialIfError(errs, r.mb.RecordApacheConnectionStatusDataPoint(now, value, newState))
	}
}

// applyCPUTimeNewFormat renames the apache.cpu.time attribute keys to the new
// format when the new format is enabled. The metric name itself does not change
// between formats, so this is handled here rather than as a separate metric.
// When both formats are enabled, new-format data points are appended alongside
// the original ones; when only the new format is enabled, the keys are renamed
// in place.
func (r *apacheScraper) applyCPUTimeNewFormat(md pmetric.Metrics) {
	if !r.emitNew {
		return
	}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() != cpuTimeMetricName || m.Type() != pmetric.MetricTypeSum {
					continue
				}
				dps := m.Sum().DataPoints()
				// Snapshot the original count because new-format points may be appended.
				original := dps.Len()
				for d := range original {
					dp := dps.At(d)
					level, hasLevel := dp.Attributes().Get(cpuTimeOldLevelKey)
					mode, hasMode := dp.Attributes().Get(cpuTimeOldModeKey)
					if !hasLevel || !hasMode {
						continue
					}
					levelVal, modeVal := level.Str(), mode.Str()
					if r.emitOld {
						// Keep the original point and add a new-format copy.
						ndp := dps.AppendEmpty()
						dp.CopyTo(ndp)
						setCPUTimeNewFormatAttributes(ndp, levelVal, modeVal)
					} else {
						// Rename the attribute keys in place.
						setCPUTimeNewFormatAttributes(dp, levelVal, modeVal)
					}
				}
			}
		}
	}
}

func setCPUTimeNewFormatAttributes(dp pmetric.NumberDataPoint, level, mode string) {
	dp.Attributes().RemoveIf(func(k string, _ pcommon.Value) bool {
		return k == cpuTimeOldLevelKey || k == cpuTimeOldModeKey
	})
	dp.Attributes().PutStr(cpuTimeNewLevelKey, level)
	dp.Attributes().PutStr(cpuTimeNewModeKey, mode)
}

func addPartialIfError(errs *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errs.AddPartial(1, err)
	}
}

// GetStats collects metric stats by making a get request at an endpoint.
func (r *apacheScraper) GetStats() (string, error) {
	resp, err := r.httpClient.Get(r.cfg.ClientConfig.Endpoint)
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

// scoreboardCountsByLabel maps a worker state (the shared enum value, e.g.
// "waiting") to the number of workers in that state.
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
