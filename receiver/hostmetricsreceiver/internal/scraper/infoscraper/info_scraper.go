// TODO: support windows
package infoscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/infoscraper"

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/infoscraper/internal/metadata"
)

const metricsLen = 1

// scraper for Load Metrics
type scraper struct {
	settings receiver.CreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder

	hostname func() (name string, err error)
	now      func() time.Time
	cpuNum   func() int
	org      func() string
	// for mocking
	bootTime func() (uint64, error)
}

func newInfoScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config) *scraper {
	return &scraper{
		settings: settings,
		config:   cfg,

		now:      time.Now,
		hostname: os.Hostname,
		cpuNum:   runtime.NumCPU,
		org:      org,

		bootTime: host.BootTime,
	}
}

func (s *scraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

// shutdown
func (s *scraper) shutdown(ctx context.Context) error {
	return nil
}

// scrape
func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(s.now())

	hostname, err := s.hostname()
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}
	s.mb.RecordInfoNowDataPoint(
		now, now.AsTime().Unix(),
		org(),
		hostname,
		int64(s.cpuNum()),
	)

	return s.mb.Emit(), nil
}

func org() string {
	return os.Getenv("EASY_ENV_COMMON_ORG")
}
