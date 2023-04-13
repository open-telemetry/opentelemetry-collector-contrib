// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type ottlStatementFilter struct {
	Queries []string
	logger  *zap.Logger
}

var _ PolicyEvaluator = (*ottlStatementFilter)(nil)

// NewOTTLStatementFilter looks at the trace data and returns a corresponding SamplingDecision.
func NewOTTLStatementFilter(logger *zap.Logger, queries []string) PolicyEvaluator {
	return &ottlStatementFilter{
		Queries: queries,
		logger:  logger,
	}
}

func (osf *ottlStatementFilter) Evaluate(_ pcommon.TraceID, trace *TraceData) (Decision, error) {
	osf.logger.Debug("Evaluating spans with OTTL query filter")
	return Sampled, nil
}
