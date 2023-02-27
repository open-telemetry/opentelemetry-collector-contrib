// Copyright The OpenTelemetry Authors
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

package supervisor

import (
	"fmt"

	"github.com/open-telemetry/opamp-go/client/types"
	"go.uber.org/zap"
)

// opAMPLogger adapts the Supervisor's zap.Logger instance so it
// can be used by the OpAMP client.
type opAMPLogger struct {
	logger *zap.Logger
}

func (l opAMPLogger) Debugf(format string, v ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, v...))
}

func (l opAMPLogger) Errorf(format string, v ...interface{}) {
	l.logger.Error(fmt.Sprintf(format, v...))
}

var _ types.Logger = opAMPLogger{}
