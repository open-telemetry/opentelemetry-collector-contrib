// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tqlconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tqlconfig"

// Config configures the TQL queries to execute against traces, metrics, and logs.
type Config struct {
	Traces  SignalConfig `mapstructure:"traces"`
	Metrics SignalConfig `mapstructure:"metrics"`
	Logs    SignalConfig `mapstructure:"logs"`
}

// SignalConfig configures TQL queries to execute.
type SignalConfig struct {
	Queries []string `mapstructure:"queries"`
}

// DeclarativeConfig configures the TQL queries to execute against traces, metrics, and logs.
type DeclarativeConfig struct {
	Traces  DeclarativeSignalConfig `mapstructure:"traces"`
	Metrics DeclarativeSignalConfig `mapstructure:"metrics"`
	Logs    DeclarativeSignalConfig `mapstructure:"logs"`
}

// DeclarativeSignalConfig configures TQL queries to execute.
type DeclarativeSignalConfig struct {
	Queries []DeclarativeQuery `mapstructure:"queries"`
}

// DeclarativeQuery configures a specific TQL query to execute.
type DeclarativeQuery struct {
	Function  string      `mapstructure:"function"`
	Arguments []Argument  `mapstructure:"arguments"`
	Condition *Expression `mapstructure:"condition"`
}

type Argument struct {
	Invocation *Invocation `mapstructure:"invocation"`
	String     *string     `mapstructure:"string"`
	Other      *string     `mapstructure:"other"`
}

type Invocation struct {
	Function  string     `mapstructure:"function"`
	Arguments []Argument `mapstructure:"arguments"`
}

type Comparison struct {
	Arguments []Argument `mapstructure:"arguments"`
	Operator  string     `mapstructure:"operator"`
}

type Expression struct {
	Comparison *Comparison  `mapstructure:"comparison"`
	And        []Expression `mapstructure:"and"`
	Or         []Expression `mapstructure:"or"`
}
