// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
