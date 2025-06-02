// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

const (
	nfsMetricsLen  = 97
	nfsdMetricsLen = 113
)

// nfsScraper for NFS Metrics
type nfsScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking
	nfsStats  func() (*NfsStats, error)
	nfsdStats func() (*NfsdStats, error)
}

// newNfsScraper creates an NFS Scraper related metric
func newNfsScraper(settings scraper.Settings, cfg *Config) *nfsScraper {
	return &nfsScraper{
		settings:  settings,
		config:    cfg,
		nfsStats:  getNfsStats,
		nfsdStats: getNfsdStats,
	}
}

func (s *nfsScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *nfsScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	var errs scrapererror.ScrapeErrors
	now := pcommon.NewTimestampFromTime(time.Now())

	nfsStats, err := s.nfsStats()
	if err == nil {
		s.recordNfsMetrics(now, nfsStats)
	} else {
		errs.AddPartial(nfsMetricsLen, err)
	}

	nfsdStats, err := s.nfsdStats()
	if err == nil {
		s.recordNfsdMetrics(now, nfsdStats)
	} else {
		errs.AddPartial(nfsdMetricsLen, err)
	}

	return s.mb.Emit(), errs.Combine()
}

func (s *nfsScraper) recordNfsMetrics(now pcommon.Timestamp, nfsStats *NfsStats) {
	if nfsStats.NfsNetStats != nil {
		s.mb.RecordSystemNfsNetCountDataPoint(now, int64(nfsStats.NfsNetStats.NetCount))
		s.mb.RecordSystemNfsNetUDPCountDataPoint(now, int64(nfsStats.NfsNetStats.UDPCount))
		s.mb.RecordSystemNfsNetTCPCountDataPoint(now, int64(nfsStats.NfsNetStats.TCPCount))
		s.mb.RecordSystemNfsNetTCPConnectionCountDataPoint(now, int64(nfsStats.NfsNetStats.TCPConnectionCount))
	}

	if nfsStats.NfsRPCStats != nil {
		s.mb.RecordSystemNfsRPCCountDataPoint(now, int64(nfsStats.NfsRPCStats.RPCCount))
		s.mb.RecordSystemNfsRPCRetransmitCountDataPoint(now, int64(nfsStats.NfsRPCStats.RetransmitCount))
		s.mb.RecordSystemNfsRPCAuthrefreshCountDataPoint(now, int64(nfsStats.NfsRPCStats.AuthRefreshCount))
	}

	if nfsStats.NfsV3ProcedureStats != nil {
		for _, callStat := range *nfsStats.NfsV3ProcedureStats {
			s.mb.RecordSystemNfsProcedureCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}

	if nfsStats.NfsV4ProcedureStats != nil {
		for _, callStat := range *nfsStats.NfsV4ProcedureStats {
			s.mb.RecordSystemNfsProcedureCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}

	if nfsStats.NfsV4OperationStats != nil {
		for _, callStat := range *nfsStats.NfsV4OperationStats {
			s.mb.RecordSystemNfsOperationCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}
}

func (s *nfsScraper) recordNfsdMetrics(now pcommon.Timestamp, nfsdStats *NfsdStats) {
	if nfsdStats.NfsdRepcacheStats != nil {
		s.mb.RecordSystemNfsdRepcacheHitsDataPoint(now, int64(nfsdStats.NfsdRepcacheStats.Hits))
		s.mb.RecordSystemNfsdRepcacheMissesDataPoint(now, int64(nfsdStats.NfsdRepcacheStats.Misses))
		s.mb.RecordSystemNfsdRepcacheNocacheDataPoint(now, int64(nfsdStats.NfsdRepcacheStats.Nocache))
	}

	if nfsdStats.NfsdFhStats != nil {
		s.mb.RecordSystemNfsdFhStaleCountDataPoint(now, int64(nfsdStats.NfsdFhStats.Stale))
	}

	if nfsdStats.NfsdIoStats != nil {
		s.mb.RecordSystemNfsdIoReadCountDataPoint(now, int64(nfsdStats.NfsdIoStats.Read))
		s.mb.RecordSystemNfsdIoWriteCountDataPoint(now, int64(nfsdStats.NfsdIoStats.Write))
	}

	if nfsdStats.NfsdThreadStats != nil {
		s.mb.RecordSystemNfsdThreadCountDataPoint(now, int64(nfsdStats.NfsdThreadStats.Threads))
	}

	if nfsdStats.NfsdNetStats != nil {
		s.mb.RecordSystemNfsdNetCountDataPoint(now, int64(nfsdStats.NfsdNetStats.NetCount))
		s.mb.RecordSystemNfsdNetUDPCountDataPoint(now, int64(nfsdStats.NfsdNetStats.UDPCount))
		s.mb.RecordSystemNfsdNetTCPCountDataPoint(now, int64(nfsdStats.NfsdNetStats.TCPCount))
		s.mb.RecordSystemNfsdNetTCPConnectionCountDataPoint(now, int64(nfsdStats.NfsdNetStats.TCPConnectionCount))
	}

	if nfsdStats.NfsdRPCStats != nil {
		s.mb.RecordSystemNfsdRPCCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.RPCCount))
		s.mb.RecordSystemNfsdRPCBadCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.BadCount))
		s.mb.RecordSystemNfsdRPCBadfmtCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.BadFmtCount))
		s.mb.RecordSystemNfsdRPCBadauthCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.BadAuthCount))
		s.mb.RecordSystemNfsdRPCBadclientCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.BadClientCount))
	}

	if nfsdStats.NfsdV3ProcedureStats != nil {
		for _, callStat := range *nfsdStats.NfsdV3ProcedureStats {
			s.mb.RecordSystemNfsdProcedureCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}

	if nfsdStats.NfsdV4ProcedureStats != nil {
		for _, callStat := range *nfsdStats.NfsdV4ProcedureStats {
			s.mb.RecordSystemNfsdProcedureCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}

	if nfsdStats.NfsdV4OperationStats != nil {
		for _, callStat := range *nfsdStats.NfsdV4OperationStats {
			s.mb.RecordSystemNfsdOperationCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}
}
