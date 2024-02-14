// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

var _ component.Config = (*Config)(nil)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	PublicKey                               string                        `mapstructure:"public_key"`
	PrivateKey                              configopaque.String           `mapstructure:"private_key"`
	Granularity                             string                        `mapstructure:"granularity"`
	MetricsBuilderConfig                    metadata.MetricsBuilderConfig `mapstructure:",squash"`
	Projects                                []*ProjectConfig              `mapstructure:"projects"`
	Alerts                                  AlertConfig                   `mapstructure:"alerts"`
	Events                                  *EventsConfig                 `mapstructure:"events"`
	Logs                                    LogConfig                     `mapstructure:"logs"`
	BackOffConfig                           configretry.BackOffConfig     `mapstructure:"retry_on_failure"`
	StorageID                               *component.ID                 `mapstructure:"storage"`
}

type AlertConfig struct {
	Enabled  bool                        `mapstructure:"enabled"`
	Endpoint string                      `mapstructure:"endpoint"`
	Secret   configopaque.String         `mapstructure:"secret"`
	TLS      *configtls.TLSServerSetting `mapstructure:"tls"`
	Mode     string                      `mapstructure:"mode"`

	// these parameters are only relevant in retrieval mode
	Projects     []*ProjectConfig `mapstructure:"projects"`
	PollInterval time.Duration    `mapstructure:"poll_interval"`
	PageSize     int64            `mapstructure:"page_size"`
	MaxPages     int64            `mapstructure:"max_pages"`
}

type LogConfig struct {
	Enabled  bool                 `mapstructure:"enabled"`
	Projects []*LogsProjectConfig `mapstructure:"projects"`
}

// EventsConfig is the configuration options for events collection
type EventsConfig struct {
	Projects      []*ProjectConfig `mapstructure:"projects"`
	Organizations []*OrgConfig     `mapstructure:"organizations"`
	PollInterval  time.Duration    `mapstructure:"poll_interval"`
	Types         []string         `mapstructure:"types"`
	PageSize      int64            `mapstructure:"page_size"`
	MaxPages      int64            `mapstructure:"max_pages"`
}

type LogsProjectConfig struct {
	ProjectConfig `mapstructure:",squash"`

	EnableAuditLogs bool              `mapstructure:"collect_audit_logs"`
	EnableHostLogs  *bool             `mapstructure:"collect_host_logs"`
	AccessLogs      *AccessLogsConfig `mapstructure:"access_logs"`
}

type AccessLogsConfig struct {
	Enabled      *bool         `mapstructure:"enabled"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
	PageSize     int64         `mapstructure:"page_size"`
	MaxPages     int64         `mapstructure:"max_pages"`
	AuthResult   *bool         `mapstructure:"auth_result"`
}

func (alc *AccessLogsConfig) IsEnabled() bool {
	return alc.Enabled == nil || *alc.Enabled
}

type ProjectConfig struct {
	Name            string   `mapstructure:"name"`
	ExcludeClusters []string `mapstructure:"exclude_clusters"`
	IncludeClusters []string `mapstructure:"include_clusters"`

	includesByClusterName map[string]struct{}
	excludesByClusterName map[string]struct{}
}

type OrgConfig struct {
	ID string `mapstructure:"id"`
}

func (pc *ProjectConfig) populateIncludesAndExcludes() {
	pc.includesByClusterName = map[string]struct{}{}
	for _, inclusion := range pc.IncludeClusters {
		pc.includesByClusterName[inclusion] = struct{}{}
	}

	pc.excludesByClusterName = map[string]struct{}{}
	for _, exclusion := range pc.ExcludeClusters {
		pc.excludesByClusterName[exclusion] = struct{}{}
	}
}

var (
	// Alerts Receiver Errors
	errNoEndpoint       = errors.New("an endpoint must be specified")
	errNoSecret         = errors.New("a webhook secret must be specified")
	errNoCert           = errors.New("tls was configured, but no cert file was specified")
	errNoKey            = errors.New("tls was configured, but no key file was specified")
	errNoModeRecognized = fmt.Errorf("alert mode not recognized for mode. Known alert modes are: %s", strings.Join([]string{
		alertModeListen,
		alertModePoll,
	}, ","))
	errPageSizeIncorrect = errors.New("page size must be a value between 1 and 500")

	// Logs Receiver Errors
	errNoProjects    = errors.New("at least one 'project' must be specified")
	errNoEvents      = errors.New("at least one 'project' or 'organizations' event type must be specified")
	errClusterConfig = errors.New("only one of 'include_clusters' or 'exclude_clusters' may be specified")

	// Access Logs Errors
	errMaxPageSize = errors.New("the maximum value for 'page_size' is 20000")
)

func (c *Config) Validate() error {
	var errs error

	for _, project := range c.Projects {
		if len(project.ExcludeClusters) != 0 && len(project.IncludeClusters) != 0 {
			errs = multierr.Append(errs, errClusterConfig)
		}
	}

	errs = multierr.Append(errs, c.Alerts.validate())
	errs = multierr.Append(errs, c.Logs.validate())
	if c.Events != nil {
		errs = multierr.Append(errs, c.Events.validate())
	}

	return errs
}

func (l *LogConfig) validate() error {
	if !l.Enabled {
		return nil
	}

	var errs error
	if len(l.Projects) == 0 {
		errs = multierr.Append(errs, errNoProjects)
	}

	for _, project := range l.Projects {
		if len(project.ExcludeClusters) != 0 && len(project.IncludeClusters) != 0 {
			errs = multierr.Append(errs, errClusterConfig)
		}

		if project.AccessLogs != nil && project.AccessLogs.IsEnabled() {
			if project.AccessLogs.PageSize > 20000 {
				errs = multierr.Append(errs, errMaxPageSize)
			}
		}
	}

	return errs
}

func (a *AlertConfig) validate() error {
	if !a.Enabled {
		// No need to further validate, receiving alerts is disabled.
		return nil
	}

	switch a.Mode {
	case alertModePoll:
		return a.validatePollConfig()
	case alertModeListen:
		return a.validateListenConfig()
	default:
		return errNoModeRecognized
	}
}

func (a AlertConfig) validatePollConfig() error {
	if len(a.Projects) == 0 {
		return errNoProjects
	}

	// based off API limits https://www.mongodb.com/docs/atlas/reference/api/alerts-get-all-alerts/
	if 0 >= a.PageSize || a.PageSize > 500 {
		return errPageSizeIncorrect
	}

	var errs error
	for _, project := range a.Projects {
		if len(project.ExcludeClusters) != 0 && len(project.IncludeClusters) != 0 {
			errs = multierr.Append(errs, errClusterConfig)
		}
	}

	return errs
}

func (a AlertConfig) validateListenConfig() error {
	if a.Endpoint == "" {
		return errNoEndpoint
	}

	var errs error
	_, _, err := net.SplitHostPort(a.Endpoint)
	if err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to split endpoint into 'host:port' pair: %w", err))
	}

	if a.Secret == "" {
		errs = multierr.Append(errs, errNoSecret)
	}

	if a.TLS != nil {
		if a.TLS.CertFile == "" {
			errs = multierr.Append(errs, errNoCert)
		}

		if a.TLS.KeyFile == "" {
			errs = multierr.Append(errs, errNoKey)
		}
	}
	return errs
}

func (e EventsConfig) validate() error {
	if len(e.Projects) == 0 && len(e.Organizations) == 0 {
		return errNoEvents
	}
	return nil
}
