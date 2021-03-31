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
	"sync"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/database"
	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
)

// LogAgent is an entity that handles log monitoring.
type LogAgent struct {
	database database.Database
	pipeline pipeline.Pipeline

	startOnce sync.Once
	stopOnce  sync.Once

	*zap.SugaredLogger
}

// Start will start the log monitoring process
func (a *LogAgent) Start() (err error) {
	a.startOnce.Do(func() {
		err = a.pipeline.Start()
		if err != nil {
			return
		}
	})
	return
}

// Stop will stop the log monitoring process
func (a *LogAgent) Stop() (err error) {
	a.stopOnce.Do(func() {
		err = a.pipeline.Stop()
		if err != nil {
			return
		}

		err = a.database.Close()
		if err != nil {
			return
		}
	})
	return
}
