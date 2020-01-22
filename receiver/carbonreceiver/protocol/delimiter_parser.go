// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package protocol

import (
	"errors"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// DelimiterParser implements a parser that can breakdown the Carbon metric path
// and transform it in corresponding metric labels.
type DelimiterParser struct {
	OrDemiliter string `mapstructure:"or_delimiter"`
}

var _ (Parser) = (*DelimiterParser)(nil)
var _ (ParserConfig) = (*DelimiterParser)(nil)

// BuildParser builds the respective parser of the configuration instance.
func (d *DelimiterParser) BuildParser() (Parser, error) {
	if d == nil {
		return &DelimiterParser{}, nil
	}
	return d, nil
}

// Parse receives the string with plaintext data, aka line, in the Carbon
// format and transforms it to the collector metric format.
func (d *DelimiterParser) Parse(line string) (*metricspb.Metric, error) {
	// TODO: for now this parser is just a place holder, implementation coming
	// soon.
	return nil, errors.New("delimiter parser not implemeted yet")
}
