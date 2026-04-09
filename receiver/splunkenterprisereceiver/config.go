// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

var (
	errBadOrMissingEndpoint = errors.New("missing a valid endpoint")
	errBadScheme            = errors.New("endpoint scheme must be either http or https")
	errMissingAuthExtension = errors.New("auth extension missing from config")

	errEmptySPL         = errors.New("search spl cannot be empty")
	errInvalidTarget    = errors.New("search target must be one of: indexer, search_head, cluster_master")
	errNoMetrics        = errors.New("search must have at least one metric defined")
	errEmptyMetricName  = errors.New("metric_name cannot be empty")
	errEmptyValueColumn = errors.New("value_column cannot be empty")
	errInvalidValueType = errors.New("value_type must be one of: int, double")
)

const (
	TargetIndexer       = "indexer"
	TargetSearchHead    = "search_head"
	TargetClusterMaster = "cluster_master"

	MetricValueTypeInt    = "int"
	MetricValueTypeDouble = "double"
)

type MetricConfig struct {
	MetricName       string            `mapstructure:"metric_name"`
	ValueColumn      string            `mapstructure:"value_column"`
	AttributeColumns []string          `mapstructure:"attribute_columns"`
	ValueType        string            `mapstructure:"value_type"`
	Unit             string            `mapstructure:"unit"`
	Description      string            `mapstructure:"description"`
	StaticAttributes map[string]string `mapstructure:"static_attributes"`
}

func (m MetricConfig) Validate() error {
	var errs error
	if m.MetricName == "" {
		errs = multierr.Append(errs, errEmptyMetricName)
	}
	if m.ValueColumn == "" {
		errs = multierr.Append(errs, errEmptyValueColumn)
	}
	if m.ValueType != "" && m.ValueType != MetricValueTypeInt && m.ValueType != MetricValueTypeDouble {
		errs = multierr.Append(errs, errInvalidValueType)
	}
	return errs
}

type SearchConfig struct {
	SPL      string         `mapstructure:"spl"`
	Target   string         `mapstructure:"target"`
	Earliest string         `mapstructure:"earliest"`
	Latest   string         `mapstructure:"latest"`
	Metrics  []MetricConfig `mapstructure:"metrics"`
}

func (s SearchConfig) Validate() error {
	var errs error
	if strings.TrimSpace(s.SPL) == "" {
		errs = multierr.Append(errs, errEmptySPL)
	}
	if s.Target != TargetIndexer && s.Target != TargetSearchHead && s.Target != TargetClusterMaster {
		errs = multierr.Append(errs, errInvalidTarget)
	}
	if len(s.Metrics) == 0 {
		errs = multierr.Append(errs, errNoMetrics)
	}
	for i, m := range s.Metrics {
		if err := m.Validate(); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("metrics[%d]: %w", i, err))
		}
	}
	return errs
}

func (s SearchConfig) TargetType() string {
	switch s.Target {
	case TargetIndexer:
		return typeIdx
	case TargetSearchHead:
		return typeSh
	case TargetClusterMaster:
		return typeCm
	default:
		return ""
	}
}

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	IdxEndpoint                    confighttp.ClientConfig `mapstructure:"indexer"`
	SHEndpoint                     confighttp.ClientConfig `mapstructure:"search_head"`
	CMEndpoint                     confighttp.ClientConfig `mapstructure:"cluster_master"`
	VersionInfo                    bool                    `mapstructure:"build_version_info"`
	Searches                       []SearchConfig          `mapstructure:"searches"`
}

func (cfg *Config) Validate() (errors error) {
	var targetURL *url.URL
	var err error
	endpoints := []string{}

	// if no endpoint is set we do not start the receiver. For each set endpoint we go through and Validate
	// that it contains an auth setting and a valid endpoint, if its missing either of these the receiver will
	// fail to start.
	if cfg.IdxEndpoint.Endpoint == "" && cfg.SHEndpoint.Endpoint == "" && cfg.CMEndpoint.Endpoint == "" {
		errors = multierr.Append(errors, errBadOrMissingEndpoint)
	} else {
		if cfg.IdxEndpoint.Endpoint != "" {
			if !cfg.IdxEndpoint.Auth.HasValue() {
				errors = multierr.Append(errors, errMissingAuthExtension)
			}
			endpoints = append(endpoints, cfg.IdxEndpoint.Endpoint)
		}
		if cfg.SHEndpoint.Endpoint != "" {
			if !cfg.SHEndpoint.Auth.HasValue() {
				errors = multierr.Append(errors, errMissingAuthExtension)
			}
			endpoints = append(endpoints, cfg.SHEndpoint.Endpoint)
		}
		if cfg.CMEndpoint.Endpoint != "" {
			if !cfg.CMEndpoint.Auth.HasValue() {
				errors = multierr.Append(errors, errMissingAuthExtension)
			}
			endpoints = append(endpoints, cfg.CMEndpoint.Endpoint)
		}

		for _, e := range endpoints {
			targetURL, err = url.Parse(e)
			if err != nil {
				errors = multierr.Append(errors, errBadOrMissingEndpoint)
				continue
			}

			// note passes for both http and https
			if !strings.HasPrefix(targetURL.Scheme, "http") {
				errors = multierr.Append(errors, errBadScheme)
			}
		}
	}

	for i, s := range cfg.Searches {
		if err := s.Validate(); err != nil {
			errors = multierr.Append(errors, fmt.Errorf("searches[%d]: %w", i, err))
		}
	}

	return errors
}
