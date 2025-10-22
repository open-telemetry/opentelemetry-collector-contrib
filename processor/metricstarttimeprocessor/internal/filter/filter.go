// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset/regexp"
)

type Filter struct {
	include filterset.FilterSet
	exclude filterset.FilterSet
}

func NewFilter(include, exclude FilterConfig) (*Filter, error) {
	var includeFilter filterset.FilterSet
	var excludeFilter filterset.FilterSet
	var err error
	if len(include.Metrics) > 0 {
		include.Config = regexConfigWithDefaults(include.Config)
		includeFilter, err = filterset.CreateFilterSet(include.Metrics, &include.Config)
		if err != nil {
			return nil, err
		}
	}
	if len(exclude.Metrics) > 0 {
		exclude.Config = regexConfigWithDefaults(exclude.Config)
		excludeFilter, err = filterset.CreateFilterSet(exclude.Metrics, &exclude.Config)
		if err != nil {
			return nil, err
		}
	}
	return &Filter{
		include: includeFilter,
		exclude: excludeFilter,
	}, nil
}

func regexConfigWithDefaults(c filterset.Config) filterset.Config {
	if c.MatchType != "regex" {
		return c
	}
	if c.RegexpConfig == nil {
		c.RegexpConfig = &regexp.Config{}
	}
	c.RegexpConfig.CacheEnabled = true
	if c.RegexpConfig.CacheMaxNumEntries == 0 {
		c.RegexpConfig.CacheMaxNumEntries = 1000 // hold 1k metric names for each cache
	}

	return c
}

func (filter *Filter) Matches(name string) bool {
	if filter.exclude != nil && filter.exclude.Matches(name) {
		return false
	}
	if filter.include != nil && !filter.include.Matches(name) {
		return false
	}
	return true
}

type FilterConfig struct {
	filterset.Config `mapstructure:",squash"`
	Metrics          []string `mapstructure:"metrics"`
}

type NoOpFilter struct {
	NoMatch bool
}

func (f NoOpFilter) Matches(_ string) bool {
	return !f.NoMatch
}
