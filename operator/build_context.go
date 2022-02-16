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

package operator

import (
	"go.uber.org/zap"
)

// BuildContext supplies contextual resources when building an operator.
type BuildContext struct {
	Logger           *zap.SugaredLogger
	DefaultOutputIDs []string
}

// WithDefaultOutputIDs sets the default output IDs for the current context or
// the current operator build
func (bc BuildContext) WithDefaultOutputIDs(ids []string) BuildContext {
	newBuildContext := bc.Copy()
	newBuildContext.DefaultOutputIDs = ids
	return newBuildContext
}

// Copy creates a copy of the build context
func (bc BuildContext) Copy() BuildContext {
	return BuildContext{
		Logger:           bc.Logger,
		DefaultOutputIDs: bc.DefaultOutputIDs,
	}
}

// NewBuildContext creates a new build context with the given logger
func NewBuildContext(lg *zap.SugaredLogger) BuildContext {
	return BuildContext{
		Logger:           lg,
		DefaultOutputIDs: []string{},
	}
}
