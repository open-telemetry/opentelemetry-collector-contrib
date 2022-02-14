// Copyright The OpenTelemetry Authors
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

package agent

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-log-collection/errors"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
)

// LogAgentBuilder is a construct used to build a log agent
type LogAgentBuilder struct {
	defaultOutput operator.Operator
	config        *Config
	logger        *zap.SugaredLogger
}

// NewBuilder creates a new LogAgentBuilder
func NewBuilder(logger *zap.SugaredLogger) *LogAgentBuilder {
	return &LogAgentBuilder{
		logger: logger,
	}
}

// WithConfig builds the agent with a given, pre-built config
func (b *LogAgentBuilder) WithConfig(cfg *Config) *LogAgentBuilder {
	b.config = cfg
	return b
}

// WithDefaultOutput adds a default output when building a log agent
func (b *LogAgentBuilder) WithDefaultOutput(defaultOutput operator.Operator) *LogAgentBuilder {
	b.defaultOutput = defaultOutput
	return b
}

// Build will build a new log agent using the values defined on the builder
func (b *LogAgentBuilder) Build() (*LogAgent, error) {
	if b.config == nil {
		return nil, errors.NewError("config must be specified", "")
	}

	if len(b.config.Pipeline) == 0 {
		return nil, errors.NewError("empty pipeline not allowed", "")
	}

	sampledLogger := b.logger.Desugar().WithOptions(
		zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(core, time.Second, 1, 10000)
		}),
	).Sugar()

	buildContext := operator.NewBuildContext(sampledLogger)

	pipeline, err := b.config.Pipeline.BuildPipeline(buildContext, b.defaultOutput)
	if err != nil {
		return nil, err
	}

	return &LogAgent{
		pipeline:      pipeline,
		SugaredLogger: b.logger,
	}, nil
}
