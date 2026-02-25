// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sematextexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter"

import (
	"github.com/influxdata/influxdb-observability/common"
	"go.uber.org/zap"
)

type zapSematextLogger struct {
	*zap.SugaredLogger
}

func newZapSematextLogger(logger *zap.Logger) common.Logger {
	return &common.ErrorLogger{
		Logger: &zapSematextLogger{
			logger.Sugar(),
		},
	}
}

func (l zapSematextLogger) Debug(msg string, kv ...any) {
	l.Debugw(msg, kv...)
}
