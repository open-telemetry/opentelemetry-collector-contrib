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

	nfsStats, err := s.nfsStats()
	if err != nil {
		errs.AddPartial(nfsMetricsLen, err)
	}

	nfsdStats, err := s.nfsdStats()
	if err != nil {
		errs.AddPartial(nfsdMetricsLen, err)
	}

	err = s.recordMetrics(&nfsStats, &nfsdStats)
	if err != nil {
		errs.AddPartial(nfsdMetricsLen, err)
	}

	return s.mb.Emit(), errs.Combine()
}

func (s *scraper) recordMetrics(nfsStats *NfsStats, nfsdStats *NfsdStats) error {
{
	now := pcommon.NewTimestampFromTime(time.Now())

	s.mb.RecordSystemNfsNetCount(now, (*nfsStats.NfsNetStats).NetCount)
	s.mb.RecordSystemNfsNetUdpCount(now, (*nfsStats.NfsNetStats).UDPCount)
	s.mb.RecordSystemNfsNetTcpCount(now, (*nfsStats.NfsNetStats).TCPCount)
	s.mb.RecordSystemNfsNetTcpConnectionCount(now, (*nfsStats.NfsNetStats).TCPConnectionCount)

	s.mb.RecordSystemNfsRpcCount(now, (*nfsStats.NFSRPCStats).RPCCount)
	s.mb.RecordSystemNfsRpcRetransmitCount(now, (*nfsStats.NFSRPCStats).RetransmitCount)
	s.mb.RecordSystemNfsRpcAuthrefreshCount(now, (*nfsStats.NFSRPCStats).AuthRefreshCount)
	
	s.mb.RecordSystemNfsProcedureCount(now, (*nfsStats.TODO))
	s.mb.RecordSystemNfsOperationCount(now, (*nfsStats.TODO))
			
	s.mb.RecordSystemNfsdRepcacheHits(now, (*nfsdStats.NfsdRepcacheStats).Hits)
	s.mb.RecordSystemNfsdRepcacheMisses(now, (*nfsdStats.NfsdRepcacheStats).Misses)
	s.mb.RecordSystemNfsdRepcacheNocache(now, (*nfsdStats.NfsdRepcacheStats).Nocache)

	s.mb.RecordSystemNfsdFhStaleCount(now, (*nfsdStats.NfsdFhStats).Stale)
	s.mb.RecordSystemNfsdIoReadCount(now, (*nfsdStats.NfsdIoStats).Read)
	s.mb.RecordSystemNfsdIoWriteCount(now, (*nfsdStats.NfsdIoStats).Write)
	s.mb.RecordSystemNfsdThreadCount(now, (*nfsdStats.NfsdThreadStats).Threads)
	s.mb.RecordSystemNfsdNetCount(now, (*nfsdStats.NfsdNetStats).NetCount)
	s.mb.RecordSystemNfsdNetUdpCount(now, (*nfsdStats.NfsdNetStats).UDPCount)
	s.mb.RecordSystemNfsdNetTcpCount(now, (*nfsdStats.NfsdNetStats).TCPCount)
	s.mb.RecordSystemNfsdNetTcpConnectionCount(now, (*nfsdStats.NfsdNetStats).TCPConnectionCount)
	
	s.mb.RecordSystemNfsdRpcCount(now, (*nfsdStats.NfsdRPCStats).RPCCount
	s.mb.RecordSystemNfsdRpcBadCount(now, (*nfsdStats.NfsdRPCSats).BadCount
	s.mb.RecordSystemNfsdRpcBadfmtCount(now, (*nfsdStats.NfsdRPCStats).BadFmtCount
	s.mb.RecordSystemNfsdRpcBadauthCount(now, (*nfsdStats.NfsdRPCStats).BadAuthCount
	s.mb.RecordSystemNfsdRpcBadclientCount(now, (*nfsdStats.NfsdRPCStats).BadClientCount
	s.mb.RecordSystemNfsdProcedureCount(now, (*nfsdStats.)
	s.mb.RecordSystemNfsdOperationCount(now, (*nfsdStats.)
}
