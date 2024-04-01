// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package journald // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// Deprecated [v0.97.0] Use Factory.CreateOperator instead.
func (c Config) Build(_ *zap.SugaredLogger) (operator.Operator, error) {
	return nil, errors.New("journald input operator is only supported on linux")
}

func createOperator(_ component.TelemetrySettings, _ component.Config) (operator.Operator, error) {
	return nil, errors.New("journald input operator is only supported on linux")
}
