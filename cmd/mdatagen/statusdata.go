// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:generate cp -r ../../distributions.yaml .distributions.yaml
//go:embed .distributions.yaml
var distrosBytes []byte

func init() {
	var dd []distroData
	err := yaml.Unmarshal(distrosBytes, &dd)
	if err != nil {
		panic(err)
	}
	for _, d := range dd {
		distros[d.Name] = d.URL
	}
}

type distroData struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// distros is a collection of distributions that can be referenced in the metadata.yaml files.
var distros = map[string]string{}

type Codeowners struct {
	// Active codeowners
	Active []string `mapstructure:"active"`
	// Emeritus codeowners
	Emeritus []string `mapstructure:"emeritus"`
}

type Status struct {
	Stability     map[string][]string `mapstructure:"stability"`
	Distributions []string            `mapstructure:"distributions"`
	Class         string              `mapstructure:"class"`
	Warnings      []string            `mapstructure:"warnings"`
	Codeowners    *Codeowners         `mapstructure:"codeowners"`
}

func (s *Status) SortedDistributions() []string {
	sorted := s.Distributions
	sort.Slice(sorted, func(i, j int) bool {
		if s.Distributions[i] == "core" {
			return true
		}
		if s.Distributions[i] == "contrib" {
			return s.Distributions[j] != "core"
		}
		if s.Distributions[j] == "core" {
			return false
		}
		if s.Distributions[j] == "contrib" {
			return s.Distributions[i] == "core"
		}
		return s.Distributions[i] < s.Distributions[j]
	})
	return sorted
}
