// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

// Config relating to Process Metric Scraper.
type Config struct {
	// MetricsBuilderConfig allows to customize scraped metrics/attributes representation.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	internal.ScraperConfig
	// Include specifies a filter on the process names that should be included from the generated metrics.
	// Exclude specifies a filter on the process names that should be excluded from the generated metrics.
	// If neither `include` or `exclude` are set, process metrics will be generated for all processes.
	Include MatchConfig `mapstructure:"include"`
	Exclude MatchConfig `mapstructure:"exclude"`

	// MuteProcessNameError is a flag that will mute the error encountered when trying to read a process the
	// collector does not have permission for.
	// See https://github.com/open-telemetry/opentelemetry-collector/issues/3004 for more information.
	MuteProcessNameError bool `mapstructure:"mute_process_name_error,omitempty"`

	// MuteProcessIOError is a flag that will mute the error encountered when trying to read IO metrics of a process
	// the collector does not have permission for.
	MuteProcessIOError bool `mapstructure:"mute_process_io_error,omitempty"`

	// ResilientProcessScraping is a flag that will let the collector continue reading a process even when
	// the collector does not have permission to read it's executable path (Linux)
	MuteProcessExeError bool `mapstructure:"mute_process_exe_error,omitempty"`

	// MuteProcessUserError is a flag that will mute the error encountered when trying to read uid which
	// doesn't exist on the system, eg. is owned by user existing in container only
	MuteProcessUserError bool `mapstructure:"mute_process_user_error,omitempty"`

	// ScrapeProcessDelay is used to indicate the minimum amount of time a process must be running
	// before metrics are scraped for it.  The default value is 0 seconds (0s)
	ScrapeProcessDelay time.Duration `mapstructure:"scrape_process_delay"`
}

type MatchConfig struct {
	filterset.Config `mapstructure:",squash"`

	Names []string `mapstructure:"names"`
}
