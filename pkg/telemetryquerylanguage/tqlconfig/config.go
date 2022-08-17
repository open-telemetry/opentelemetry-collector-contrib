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
// Function is the name of the function and is required for interpretation to work correctly.
// Arguments are the arguments that are passed to the function and is optional.
// Condition determines when the function should be called and is option. When not supplied the function will always be called.
type DeclarativeQuery struct {
	Function  string      `mapstructure:"function"`
	Arguments []Argument  `mapstructure:"arguments"`
	Condition *Expression `mapstructure:"condition"`
}

// Argument represents an argument of a function or condition.
// Factory should be used to represent a factory function call.
// String should be used to represent a string.  The value of String will be wrapped in "" during interpretation.
// Other should be used for all other types.
type Argument struct {
	Factory *Factory `mapstructure:"factory"`
	String  *string  `mapstructure:"string"`
	Other   *string  `mapstructure:"other"`
}

// Factory should be used to represent a factory function call.
// Function is the name of the function and is required for interpretation to work correctly.
// Arguments are the arguments that are passed to the function and is optional.
type Factory struct {
	Function  string     `mapstructure:"function"`
	Arguments []Argument `mapstructure:"arguments"`
}

// Expression represents a single, or group, of comparisons.
// Whenever using an Expression, the expectation is that exactly one of Comparison, And, or Or is used.
// Comparison represents a single comparison.
// And represents a group of comparisons ANDed together.
// Or represents a group of comparisons ORed together.
type Expression struct {
	Comparison *Comparison  `mapstructure:"comparison"`
	And        []Expression `mapstructure:"and"`
	Or         []Expression `mapstructure:"or"`
}

// Comparison represents a logical comparison.
// Arguments is a list of exactly 2 Arguments, which represent the left and right side of a comparison.
// Operator is the operator that compares the 2 arguments.
type Comparison struct {
	Arguments []Argument `mapstructure:"arguments"`
	Operator  string     `mapstructure:"operator"`
}
