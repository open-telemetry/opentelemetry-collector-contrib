// TODO: support windows
package infoscraper

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/infoscraper/internal/metadata"
)

func Test_newInfoScraper(t *testing.T) {
	type args struct {
		settings receiver.CreateSettings
		cfg      *Config
	}
	tests := []struct {
		name string
		args args
		want *scraper
	}{
		{
			args: args{},
			want: &scraper{
				now:      time.Now,
				hostname: os.Hostname,
				cpuNum:   runtime.NumCPU,
				org:      org,
				bootTime: host.BootTime,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newInfoScraper(context.Background(), tt.args.settings, tt.args.cfg)
			assert.IsType(t, got, tt.want)
		})
	}
}

func Test_scraper_start(t *testing.T) {
	type args struct {
		ctx context.Context
		in1 component.Host
	}
	tests := []struct {
		name    string
		s       *scraper
		args    args
		wantErr bool
	}{
		{
			name: "success",
			s: &scraper{
				bootTime: func() (uint64, error) {
					return 1, nil
				},
				config: &Config{},
			},
			wantErr: false,
		},
		{
			name: "failed",
			s: &scraper{
				bootTime: func() (uint64, error) {
					return 0, fmt.Errorf("mock err")
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.s.start(tt.args.ctx, tt.args.in1); (err != nil) != tt.wantErr {
				t.Errorf("scraper.start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_scraper_shutdown(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		s       *scraper
		args    args
		wantErr bool
	}{
		{
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.s.shutdown(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("scraper.shutdown() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_scraper_scrape(t *testing.T) {
	tests := []struct {
		name    string
		s       *scraper
		want    pmetric.Metrics
		wantErr bool
	}{
		{
			name: "failed",
			s: &scraper{
				now:    time.Now,
				config: &Config{},
				bootTime: func() (uint64, error) {
					return 1, nil
				},
				hostname: func() (name string, err error) {
					return "", fmt.Errorf("mock err")
				},
			},
			wantErr: true,
		},
		{
			name: "success",
			s: &scraper{
				settings: receivertest.NewNopCreateSettings(),
				now:    time.Now,
				config: &Config{
					MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				},
				bootTime: func() (uint64, error) {
					return 1, nil
				},
				hostname: func() (name string, err error) {
					return "", nil
				},
				cpuNum: runtime.NumCPU,
				org:    org,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.s.start(context.Background(), nil)
			got, err := tt.s.scrape(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("scraper.scrape() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.Equal(t, 1, got.MetricCount())
			}
		})
	}
}

func Test_org(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := org(); got != tt.want {
				t.Errorf("org() = %v, want %v", got, tt.want)
			}
		})
	}
}
