// Copyright  OpenTelemetry Authors
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

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

import (
	"context"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

var _ component.TracesProcessor = (*redaction)(nil)

type redaction struct {
	// Attribute keys allowed in a span
	allowList map[string]string
	// Attribute values blocked in a span
	blockRegexList map[string]*regexp.Regexp
	// Redaction processor configuration
	config *Config
	// Logger
	logger *zap.Logger
	// Next trace consumer in line
	next consumer.Traces
}

// newRedaction creates a new instance of the redaction processor
func newRedaction(ctx context.Context, config *Config, logger *zap.Logger, next consumer.Traces) (*redaction, error) {
	allowList := makeAllowList(config)
	blockRegexList, err := makeBlockRegexList(ctx, config)
	if err != nil {
		// TODO: Placeholder for an error metric in the next PR
		return nil, fmt.Errorf("failed to process block list: %w", err)
	}

	return &redaction{
		allowList:      allowList,
		blockRegexList: blockRegexList,
		config:         config,
		logger:         logger,
		next:           next,
	}, nil
}

// processTraces implements ProcessMetricsFunc. It processes the incoming data
// and returns the data to be sent to the next component
func (s *redaction) processTraces(_ context.Context, batch pdata.Traces) (pdata.Traces, error) {
	// TODO: Implementation to follow in the next PR
	return batch, nil
}

// ConsumeTraces implements the SpanProcessor interface
func (s *redaction) ConsumeTraces(_ context.Context, _ pdata.Traces) error {
	// TODO: Implementation to follow in the next PR
	return nil
}

const (
	redactedKeys     = "redaction.redacted.keys"
	redactedKeyCount = "redaction.redacted.count"
	maskedValues     = "redaction.masked.keys"
	maskedValueCount = "redaction.masked.count"
)

// makeAllowList sets up a lookup table of allowed span attribute keys
func makeAllowList(c *Config) map[string]string {
	// redactionKeys are additional span attributes created by the processor to
	// summarize the changes it made to a span. If the processor removes
	// 2 attributes from a span (e.g. `birth_date`, `mothers_maiden_name`),
	// then it will list them in the `redaction.redacted.keys` span attribute
	// and set the `redaction.redacted.count` attribute to 2
	//
	// If the processor finds and masks values matching a blocked regex in 2
	// span attributes (e.g. `notes`, `description`), then it will those
	// attribute keys in `redaction.masked.keys` and set the
	// `redaction.masked.count` to 2
	redactionKeys := []string{redactedKeys, redactedKeyCount, maskedValues, maskedValueCount}
	// allowList consists of the keys explicitly allowed by the configuration
	// as well as of the new span attributes that the processor creates to
	// summarize its changes
	allowList := make(map[string]string, len(c.AllowedKeys)+len(redactionKeys))
	for _, key := range c.AllowedKeys {
		allowList[key] = key
	}
	for _, key := range redactionKeys {
		allowList[key] = key
	}
	return allowList
}

// makeBlockRegexList precompiles all the blocked regex patterns
func makeBlockRegexList(_ context.Context, config *Config) (map[string]*regexp.Regexp, error) {
	blockRegexList := make(map[string]*regexp.Regexp, len(config.BlockedValues))
	for _, pattern := range config.BlockedValues {
		re, err := regexp.Compile(pattern)
		if err != nil {
			// TODO: Placeholder for an error metric in the next PR
			return nil, fmt.Errorf("error compiling regex in block list: %w", err)
		}
		blockRegexList[pattern] = re
	}
	return blockRegexList, nil
}

// Capabilities specifies what this processor does, such as whether it mutates data
func (s *redaction) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start the redaction processor
func (s *redaction) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown the redaction processor
func (s *redaction) Shutdown(context.Context) error {
	return nil
}
