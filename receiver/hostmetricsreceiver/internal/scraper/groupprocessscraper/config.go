// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupprocessscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/groupprocessscraper/internal/metadata"
)

// Config relating to Process Metric Scraper.
type Config struct {
	// MetricsBuilderConfig allows to customize scraped metrics/attributes representation.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	internal.ScraperConfig

	GroupConfig []GroupMatchConfig `mapstructure:"process_configs"`

	// MuteProcessAllErrors is a flag that will mute all the errors encountered when trying to read metrics of a process.
	// When this flag is enabled, there is no need to activate any other error suppression flags.
	MuteProcessAllErrors bool `mapstructure:"mute_process_all_errors,omitempty"`

	// MuteProcessNameError is a flag that will mute the error encountered when trying to read a process name the
	// collector does not have permission to read.
	// See https://github.com/open-telemetry/opentelemetry-collector/issues/3004 for more information.
	// This flag is ignored when MuteProcessAllErrors is set to true as all errors are muted.
	MuteProcessNameError bool `mapstructure:"mute_process_name_error,omitempty"`

	// MuteProcessIOError is a flag that will mute the error encountered when trying to read IO metrics of a process
	// the collector does not have permission to read.
	// This flag is ignored when MuteProcessAllErrors is set to true as all errors are muted.
	MuteProcessIOError bool `mapstructure:"mute_process_io_error,omitempty"`

	// MuteProcessCgroupError is a flag that will mute the error encountered when trying to read the cgroup of a process
	// the collector does not have permission to read.
	// This flag is ignored when MuteProcessAllErrors is set to true as all errors are muted.
	MuteProcessCgroupError bool `mapstructure:"mute_process_cgroup_error,omitempty"`

	// MuteProcessExeError is a flag that will mute the error encountered when trying to read the executable path of a process
	// the collector does not have permission to read (Linux).
	// This flag is ignored when MuteProcessAllErrors is set to true as all errors are muted.
	MuteProcessExeError bool `mapstructure:"mute_process_exe_error,omitempty"`

	// MuteProcessUserError is a flag that will mute the error encountered when trying to read uid which
	// doesn't exist on the system, eg. is owned by user existing in container only.
	// This flag is ignored when MuteProcessAllErrors is set to true as all errors are muted.
	MuteProcessUserError bool `mapstructure:"mute_process_user_error,omitempty"`

	// ScrapeProcessDelay is used to indicate the minimum amount of time a process must be running
	// before metrics are scraped for it.  The default value is 0 seconds (0s).
	ScrapeProcessDelay time.Duration `mapstructure:"scrape_process_delay"`
}

type MatchConfig struct {
	Names     []string `mapstructure:"names"`
	MatchType string   `mapstructure:"match_type"`
}

type GroupMatchConfig struct {
	GroupName string      `mapstructure:"group_name"`
	Comm      MatchConfig `mapstructure:"comm"`
	Exe       MatchConfig `mapstructure:"exe"`
	Cmdline   MatchConfig `mapstructure:"cmdline"`
}
